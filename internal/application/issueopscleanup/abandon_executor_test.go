package issueopscleanup

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type abandonTestRecords struct {
	snapshot             model.CleanupSnapshot
	events               *[]string
	armErr, checkErr     error
	deleteErr            error
	armed                *model.IssueOpsCleanupAbandonFailure
	failed               *model.IssueOpsCleanupAbandonFailure
	failedDrained        bool
	failContextCancelled bool
	outcome              model.IssueOpsCycleOutcome
}

func (s *abandonTestRecords) Load(context.Context, string) (model.CleanupSnapshot, error) {
	*s.events = append(*s.events, "load")
	return s.snapshot, nil
}
func (s *abandonTestRecords) ArmAbandon(_ context.Context, snap model.CleanupSnapshot, _ model.IssueOpsCleanupAttempt, f model.IssueOpsCleanupAbandonFailure) (model.CleanupSnapshot, error) {
	*s.events = append(*s.events, "arm")
	s.armed = &f
	return snap, s.armErr
}
func (s *abandonTestRecords) Check(context.Context, model.CleanupSnapshot) error {
	*s.events = append(*s.events, "check")
	return s.checkErr
}
func (s *abandonTestRecords) FailAbandon(ctx context.Context, snap model.CleanupSnapshot, f model.IssueOpsCleanupAbandonFailure, drained bool) (model.CleanupSnapshot, error) {
	*s.events = append(*s.events, "fail")
	s.failed, s.failedDrained = &f, drained
	s.failContextCancelled = ctx.Err() != nil
	return snap, nil
}
func (s *abandonTestRecords) DeleteAbandoned(_ context.Context, _ model.CleanupSnapshot, outcome model.IssueOpsCycleOutcome) ([]string, error) {
	s.outcome = outcome
	*s.events = append(*s.events, "delete")
	return []string{"intent-1"}, s.deleteErr
}

type abandonTestProvider struct {
	port.IssueProvider
	events      *[]string
	closePR     port.IssueProviderClosePullRequestResult
	closePRErr  error
	closeIssue  port.IssueProviderCloseIssueResult
	artifact    port.IssueProviderArtifactBody
	artifactErr error
	gotKind     string
}

func (p *abandonTestProvider) ClosePullRequest(context.Context, port.IssueProviderClosePullRequestRequest) (port.IssueProviderClosePullRequestResult, error) {
	*p.events = append(*p.events, "close_pr")
	return p.closePR, p.closePRErr
}
func (p *abandonTestProvider) CloseIssue(context.Context, port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	*p.events = append(*p.events, "close_issue")
	return p.closeIssue, nil
}
func (p *abandonTestProvider) ReadArtifactBody(_ context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	p.gotKind = req.Kind
	return p.artifact, p.artifactErr
}

type abandonTestFixture struct {
	executor  AbandonExecutor
	records   *abandonTestRecords
	lease     *finishTestLease
	events    *[]string
	inventory *model.CleanupAbandonInventory
	result    *model.CleanupAbandonResult
	provider  *abandonTestProvider
	gitCode   map[string]int
}

func abandonExecutorFixture() *abandonTestFixture {
	events := []string{}
	f := &abandonTestFixture{events: &events, gitCode: map[string]int{}}
	f.lease = &finishTestLease{events: &events}
	f.records = &abandonTestRecords{events: &events, snapshot: model.CleanupSnapshot{Record: model.IssueOpsRecord{ID: "io-abandon", Repo: "/repo", BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "github"}}}}
	f.inventory = &model.CleanupAbandonInventory{WorktreePresent: true, WorktreeRoot: "/worktree", Branch: "feature", BranchOID: "oid"}
	f.result = &model.CleanupAbandonResult{OK: true, ID: "io-abandon", Reason: "no longer needed"}
	f.provider = &abandonTestProvider{events: &events}
	f.executor = AbandonExecutor{
		Records: f.records,
		Acquire: func(context.Context, string) (CleanupLifetime, error) {
			events = append(events, "acquire")
			return f.lease, nil
		},
		Provider: func(string) (port.IssueProvider, error) { return f.provider, nil },
		Observe: func(_ context.Context, _ model.IssueOpsRecord, r model.CleanupAbandonRequest, _ port.IssueProvider) (model.CleanupAbandonRequest, error) {
			events = append(events, "observe")
			return r, nil
		},
		Plan: func(context.Context, model.IssueOpsRecord, model.CleanupAbandonRequest, port.IssueProvider) (model.CleanupAbandonInventory, model.CleanupAbandonResult) {
			events = append(events, "plan")
			return *f.inventory, *f.result
		},
		NewAttempt: func(model.CleanupOperation) (model.IssueOpsCleanupAttempt, error) {
			return model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationAbandon, Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}, nil
		},
		Stop: func(context.Context, model.CleanupAbandonInventory, []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, error) {
			events = append(events, "stop")
			return []model.CleanupWorkspaceProcess{{PID: 7, Command: "sh"}}, nil
		},
		Directory: func(string) (bool, error) { events = append(events, "directory"); return true, nil },
		Git: func(_ context.Context, _ string, args ...string) (int, string) {
			joined := strings.Join(args, " ")
			events = append(events, joined)
			return f.gitCode[args[0]], "git output"
		},
		Now: func() time.Time { return time.Date(2026, 9, 29, 0, 0, 1, 0, time.UTC) },
	}
	return f
}

func (f *abandonTestFixture) applyRequest(t *testing.T) model.CleanupAbandonRequest {
	t.Helper()
	preview, err := f.executor.Run(context.Background(), model.CleanupAbandonRequest{ID: "io-abandon"})
	if err != nil || preview.Fingerprint == "" {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	*f.events = (*f.events)[:0]
	return model.CleanupAbandonRequest{ID: "io-abandon", Apply: true, Confirm: true, Fingerprint: preview.Fingerprint}
}

func TestAbandonExecutorPreviewReportsFingerprintCommandWithoutArming(t *testing.T) {
	f := abandonExecutorFixture()
	result, err := f.executor.Run(context.Background(), model.CleanupAbandonRequest{ID: "io-abandon", ClosePR: true})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := CleanupAbandonFingerprint(*f.inventory)
	if result.Fingerprint != want || result.Record == nil || result.RecordDeleted {
		t.Fatalf("result=%+v", result)
	}
	for _, part := range []string{"--id io-abandon", `--reason "no longer needed"`, "--close-pr", "--apply --confirm --fingerprint " + want} {
		if !strings.Contains(result.NextCommand, part) {
			t.Fatalf("next command %q missing %q", result.NextCommand, part)
		}
	}
	if got, wantEvents := *f.events, []string{"acquire", "load", "observe", "plan", "close"}; !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("events=%v want=%v", got, wantEvents)
	}
}

func TestAbandonExecutorRefusesNotReadyPlan(t *testing.T) {
	f := abandonExecutorFixture()
	f.result.Missing = []string{"clean worktree", "branch oid"}
	result, err := f.executor.Run(context.Background(), model.CleanupAbandonRequest{ID: "io-abandon", Apply: true, Confirm: true})
	if err == nil || !strings.Contains(err.Error(), "cleanup abandon is not ready: clean worktree, branch oid") {
		t.Fatalf("err=%v", err)
	}
	if !reflect.DeepEqual(result.Missing, f.result.Missing) {
		t.Fatalf("result=%+v", result)
	}
	for _, e := range *f.events {
		if e == "arm" {
			t.Fatalf("armed a not-ready plan: %v", *f.events)
		}
	}
}

func TestAbandonExecutorApplyRequiresConfirmAndCurrentFingerprint(t *testing.T) {
	f := abandonExecutorFixture()
	req := f.applyRequest(t)

	noConfirm := req
	noConfirm.Confirm = false
	if _, err := f.executor.Run(context.Background(), noConfirm); err == nil || !strings.Contains(err.Error(), "requires --confirm") {
		t.Fatalf("err=%v", err)
	}
	stale := req
	stale.Fingerprint = "stale"
	result, err := f.executor.Run(context.Background(), stale)
	if err == nil || !strings.Contains(err.Error(), "stale cleanup fingerprint") || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, e := range *f.events {
		if e == "arm" || e == "delete" {
			t.Fatalf("destructive step ran after rejected apply: %v", *f.events)
		}
	}
}

func TestAbandonExecutorApplyOrdersEffectsDrainsBeforeDelete(t *testing.T) {
	f := abandonExecutorFixture()
	f.result.WorkspaceProcesses = []model.CleanupWorkspaceProcess{{PID: 7, Command: "sh"}}
	req := f.applyRequest(t)
	result, err := f.executor.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RecordDeleted || !result.WorktreeRemoved || !result.BranchDeleted || len(result.WorkspaceProcessesStopped) != 1 || result.AbandonedAt != "2026-09-29T00:00:01Z" || !reflect.DeepEqual(result.IntentRowsDeleted, []string{"intent-1"}) {
		t.Fatalf("result=%+v", result)
	}
	want := []string{"acquire", "load", "observe", "plan", "arm", "check", "stop", "check", "worktree remove /worktree", "check", "update-ref -d refs/heads/feature oid", "drain", "delete", "close"}
	if !reflect.DeepEqual(*f.events, want) {
		t.Fatalf("events=%v want=%v", *f.events, want)
	}
	if f.records.armed == nil || f.records.armed.Step != model.CleanupFailureStepApplying || f.records.armed.Fingerprint != req.Fingerprint {
		t.Fatalf("armed=%+v", f.records.armed)
	}
	if got := f.records.outcome; got.Outcome != model.IssueOpsCycleOutcomeAbandoned || got.ID != req.ID || got.RetainedAt != "2026-09-29T00:00:01Z" {
		t.Fatalf("retained outcome = %+v", got)
	}
}

func TestAbandonExecutorMapsLocalFailuresToStepsAndPreservesRecord(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*abandonTestFixture)
		step  string
	}{
		{"worktree remove", func(f *abandonTestFixture) { f.gitCode["worktree"] = 1 }, model.CleanupFailureStepWorktreeRemove},
		{"branch delete", func(f *abandonTestFixture) { f.gitCode["update-ref"] = 1; f.gitCode["show-ref"] = 0 }, model.CleanupFailureStepBranchDelete},
		{"drain", func(f *abandonTestFixture) { f.lease.drainErr = errors.New("child alive") }, model.CleanupFailureStepRecordDelete},
		{"record delete", func(f *abandonTestFixture) { f.records.deleteErr = errors.New("db locked") }, model.CleanupFailureStepRecordDelete},
		{"process stop check", func(f *abandonTestFixture) {
			f.result.WorkspaceProcesses = []model.CleanupWorkspaceProcess{{PID: 7, Command: "sh"}}
			f.records.checkErr = errors.New("record drifted")
		}, model.CleanupFailureStepWorkspaceProcessesStop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := abandonExecutorFixture()
			tc.setup(f)
			req := f.applyRequest(t)
			result, err := f.executor.Run(context.Background(), req)
			if err == nil || !strings.Contains(err.Error(), "cleanup abandon "+tc.step+" failed (record preserved") {
				t.Fatalf("err=%v", err)
			}
			if result.OK || result.RecordDeleted || result.FailedStep != tc.step || !strings.Contains(result.NextCommand, "--preview") {
				t.Fatalf("result=%+v", result)
			}
			if f.records.failed == nil || f.records.failed.Step != tc.step || f.records.failed.InventorySHA256 == "" {
				t.Fatalf("failure receipt=%+v", f.records.failed)
			}
			undrained := tc.name == "drain"
			if f.records.failedDrained == undrained {
				t.Fatalf("drained=%v for %s", f.records.failedDrained, tc.name)
			}
		})
	}
}

func TestAbandonExecutorTreatsVanishedWorktreeAndBranchAsRemoved(t *testing.T) {
	f := abandonExecutorFixture()
	f.gitCode["worktree"] = 1
	f.gitCode["update-ref"] = 1
	f.gitCode["show-ref"] = 1 // ref no longer exists
	f.executor.Directory = func(string) (bool, error) { return false, nil }
	req := f.applyRequest(t)
	result, err := f.executor.Run(context.Background(), req)
	if err != nil || !result.WorktreeRemoved || !result.BranchDeleted || !result.RecordDeleted {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAbandonRunCancelledLocalRemovalKeepsApplyingSeal(t *testing.T) {
	f := abandonExecutorFixture()
	ctx, cancel := context.WithCancel(context.Background())
	f.executor.Git = func(context.Context, string, ...string) (int, string) {
		cancel()
		return 1, "signal: killed"
	}
	req := f.applyRequest(t)
	result, err := f.executor.Run(ctx, req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if result.FailedStep != model.CleanupFailureStepWorktreeRemove {
		t.Fatalf("result=%+v", result)
	}
	if f.records.failed == nil || f.records.failed.Step != model.CleanupFailureStepApplying {
		t.Fatalf("receipt must stay applying after cancellation: %+v", f.records.failed)
	}
	if f.records.failContextCancelled {
		t.Fatal("failure receipt must be written with a context detached from cancellation")
	}
	if !f.records.failedDrained {
		t.Fatal("lifetime must still drain after cancellation")
	}
}

func TestAbandonExecutorRemoteEffectsRunBeforeLocalAndAreReported(t *testing.T) {
	f := abandonExecutorFixture()
	f.records.snapshot.Record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Kind: "pr", URL: "https://example/pr/1"}
	f.inventory.RemoteBranchOID = "remote-oid"
	f.provider.closePR = port.IssueProviderClosePullRequestResult{Closed: true, AlreadyClosed: true, State: "closed"}
	f.provider.closeIssue = port.IssueProviderCloseIssueResult{Closed: true, State: "closed"}
	f.provider.artifact = port.IssueProviderArtifactBody{State: "open"}
	preview := model.CleanupAbandonRequest{ID: "io-abandon", ClosePR: true, CloseIssue: true, DeleteRemoteBranch: true}
	res, err := f.executor.Run(context.Background(), preview)
	if err != nil {
		t.Fatal(err)
	}
	*f.events = (*f.events)[:0]
	apply := preview
	apply.Apply, apply.Confirm, apply.Fingerprint = true, true, res.Fingerprint
	result, err := f.executor.Run(context.Background(), apply)
	if err != nil {
		t.Fatal(err)
	}
	wantEffects := []string{"close_pr:already_closed", "close_issue", "remote_branch_delete"}
	if !reflect.DeepEqual(result.RemoteEffects, wantEffects) || !result.PRClosed || !result.IssueClosed || !result.RemoteBranchDeleted || result.RemoteArtifactState != "closed" || result.IssueState != "closed" {
		t.Fatalf("result=%+v", result)
	}
	var order []string
	for _, e := range *f.events {
		if e == "close_pr" || e == "close_issue" || strings.HasPrefix(e, "push") || strings.HasPrefix(e, "worktree") {
			order = append(order, strings.Fields(e)[0])
		}
	}
	if want := []string{"close_pr", "close_issue", "push", "worktree"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order=%v want=%v", order, want)
	}
	if !contains(*f.events, "push origin --delete refs/heads/feature --force-with-lease=refs/heads/feature:remote-oid") {
		t.Fatalf("events=%v", *f.events)
	}
}

func TestAbandonExecutorRemoteBranchAbsentIsRecordedWithoutPush(t *testing.T) {
	f := abandonExecutorFixture()
	preview := model.CleanupAbandonRequest{ID: "io-abandon", DeleteRemoteBranch: true}
	res, err := f.executor.Run(context.Background(), preview)
	if err != nil {
		t.Fatal(err)
	}
	*f.events = (*f.events)[:0]
	preview.Apply, preview.Confirm, preview.Fingerprint = true, true, res.Fingerprint
	result, err := f.executor.Run(context.Background(), preview)
	if err != nil || result.RemoteBranchDeleted || !reflect.DeepEqual(result.RemoteEffects, []string{"remote_branch_delete:absent"}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, e := range *f.events {
		if strings.HasPrefix(e, "push") {
			t.Fatalf("pushed although remote branch absent: %v", *f.events)
		}
	}
}

func TestAbandonExecutorRemoteFailuresStopBeforeLocalEffects(t *testing.T) {
	cases := []struct {
		name  string
		req   model.CleanupAbandonRequest
		setup func(*abandonTestFixture)
		step  string
	}{
		{"merged pr", model.CleanupAbandonRequest{ClosePR: true}, func(f *abandonTestFixture) {
			f.provider.closePR = port.IssueProviderClosePullRequestResult{Merged: true}
		}, model.CleanupFailureStepClosePR},
		{"close pr error", model.CleanupAbandonRequest{ClosePR: true}, func(f *abandonTestFixture) { f.provider.closePRErr = errors.New("api down") }, model.CleanupFailureStepClosePR},
		{"close issue drift", model.CleanupAbandonRequest{CloseIssue: true}, func(f *abandonTestFixture) { f.records.checkErr = errors.New("drift") }, model.CleanupFailureStepCloseIssue},
		{"push rejected", model.CleanupAbandonRequest{DeleteRemoteBranch: true}, func(f *abandonTestFixture) { f.inventory.RemoteBranchOID = "r"; f.gitCode["push"] = 1 }, model.CleanupFailureStepRemoteBranchDelete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := abandonExecutorFixture()
			f.records.snapshot.Record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Kind: "pr", URL: "u"}
			f.provider.artifact = port.IssueProviderArtifactBody{State: "open"}
			tc.setup(f)
			preview := tc.req
			preview.ID = "io-abandon"
			res, err := f.executor.Run(context.Background(), preview)
			if err != nil {
				t.Fatal(err)
			}
			*f.events = (*f.events)[:0]
			apply := preview
			apply.Apply, apply.Confirm, apply.Fingerprint = true, true, res.Fingerprint
			result, err := f.executor.Run(context.Background(), apply)
			if err == nil || result.FailedStep != tc.step || result.RecordDeleted || result.WorktreeRemoved {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			for _, e := range *f.events {
				if strings.HasPrefix(e, "worktree") || strings.HasPrefix(e, "update-ref") || e == "delete" {
					t.Fatalf("local effect after remote failure: %v", *f.events)
				}
			}
		})
	}
}

func TestAbandonExecutorRequiresResolvableProviderForRemoteEvidence(t *testing.T) {
	f := abandonExecutorFixture()
	f.records.snapshot.Record.BranchPrepare = nil
	_, err := f.executor.Run(context.Background(), model.CleanupAbandonRequest{ID: "io-abandon", CloseIssue: true})
	if err == nil || !strings.Contains(err.Error(), "cannot determine provider") {
		t.Fatalf("err=%v", err)
	}
	f = abandonExecutorFixture()
	f.executor.Provider = func(string) (port.IssueProvider, error) { return nil, errors.New("no token") }
	if _, err := f.executor.Run(context.Background(), model.CleanupAbandonRequest{ID: "io-abandon", CloseIssue: true}); err == nil || err.Error() != "no token" {
		t.Fatalf("err=%v", err)
	}
}

func TestAbandonExecutorResolvesProviderOnlyWhenRemoteEvidenceIsNeeded(t *testing.T) {
	artifact := &model.IssueOpsRemoteArtifactVerification{Kind: "pr", URL: "u"}
	cases := []struct {
		name      string
		artifact  *model.IssueOpsRemoteArtifactVerification
		req       model.CleanupAbandonRequest
		wantCalls int
	}{
		{"local only needs no provider", nil, model.CleanupAbandonRequest{}, 0},
		{"recorded artifact needs a provider", artifact, model.CleanupAbandonRequest{}, 1},
		{"remote flag needs a provider", nil, model.CleanupAbandonRequest{DeleteRemoteBranch: true}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := abandonExecutorFixture()
			f.records.snapshot.Record.RemoteArtifact = tc.artifact
			calls := 0
			f.executor.Provider = func(string) (port.IssueProvider, error) {
				calls++
				if tc.wantCalls == 0 {
					return nil, errors.New("provider must not be resolved")
				}
				return f.provider, nil
			}
			var observed port.IssueProvider
			f.executor.Observe = func(_ context.Context, _ model.IssueOpsRecord, r model.CleanupAbandonRequest, p port.IssueProvider) (model.CleanupAbandonRequest, error) {
				observed = p
				return r, nil
			}
			req := tc.req
			req.ID = "io-abandon"
			if _, err := f.executor.Run(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if calls != tc.wantCalls || (observed != nil) != (tc.wantCalls == 1) {
				t.Fatalf("provider calls=%d observed=%v want calls=%d", calls, observed, tc.wantCalls)
			}
		})
	}
}

func TestAbandonRunKeepsApplyingSealOnlyWhenLocalRemovalsAreCancelled(t *testing.T) {
	cases := []struct {
		name        string
		setup       func(f *abandonTestFixture, cancel context.CancelFunc)
		wantFailed  string
		wantReceipt string
	}{
		{"branch delete cancelled", func(f *abandonTestFixture, cancel context.CancelFunc) {
			f.executor.Git = func(_ context.Context, _ string, args ...string) (int, string) {
				if args[0] == "update-ref" {
					cancel()
					return 1, "signal: killed"
				}
				return 0, ""
			}
		}, model.CleanupFailureStepBranchDelete, model.CleanupFailureStepApplying},
		{"record delete drain cancelled", func(f *abandonTestFixture, _ context.CancelFunc) {
			f.lease.drainErr = context.Canceled
		}, model.CleanupFailureStepRecordDelete, model.CleanupFailureStepRecordDelete},
		{"process stop cancelled", func(f *abandonTestFixture, _ context.CancelFunc) {
			f.result.WorkspaceProcesses = []model.CleanupWorkspaceProcess{{PID: 7, Command: "sh"}}
			f.records.checkErr = context.Canceled
		}, model.CleanupFailureStepWorkspaceProcessesStop, model.CleanupFailureStepWorkspaceProcessesStop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := abandonExecutorFixture()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tc.setup(f, cancel)
			req := f.applyRequest(t)
			result, err := f.executor.Run(ctx, req)
			if err == nil || result.FailedStep != tc.wantFailed {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if f.records.failed == nil || f.records.failed.Step != tc.wantReceipt {
				t.Fatalf("receipt=%+v want step %q", f.records.failed, tc.wantReceipt)
			}
		})
	}
}

func TestAbandonExecutorStopsWorkspaceOnlyWhenSomethingOccupiesIt(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(*abandonTestFixture)
		wantStop bool
	}{
		{"nothing to stop", func(*abandonTestFixture) {}, false},
		{"processes only", func(f *abandonTestFixture) {
			f.result.WorkspaceProcesses = []model.CleanupWorkspaceProcess{{PID: 7, Command: "sh"}}
		}, true},
		{"worktree already gone", func(f *abandonTestFixture) {
			f.inventory.WorktreePresent = false
			f.result.WorkspaceProcesses = []model.CleanupWorkspaceProcess{{PID: 7, Command: "sh"}}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := abandonExecutorFixture()
			tc.setup(f)
			req := f.applyRequest(t)
			if _, err := f.executor.Run(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if got := contains(*f.events, "stop"); got != tc.wantStop {
				t.Fatalf("stop called=%v want %v events=%v", got, tc.wantStop, *f.events)
			}
		})
	}
}

func TestAbandonExecutorRejectsCycleOwnedByAnotherCleanup(t *testing.T) {
	f := abandonExecutorFixture()
	f.records.snapshot.Record.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationFinish}
	_, err := f.executor.Run(context.Background(), model.CleanupAbandonRequest{ID: "io-abandon"})
	if err == nil || !strings.Contains(err.Error(), "resume cleanup finish") {
		t.Fatalf("err=%v", err)
	}
}

func TestObserveAbandonArtifactDerivesUnmergedFromLiveState(t *testing.T) {
	cases := []struct {
		kind, state  string
		wantUnmerged bool
		wantKind     string
		wantErr      string
	}{
		{"pull_request", "OPEN", true, "pr", ""},
		{"merge_request", " opened ", true, "mr", ""},
		{"pr", "closed", true, "pr", ""},
		{"pr", "merged", false, "pr", ""},
		{"pr", "weird", false, "pr", "state is unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.kind+"/"+tc.state, func(t *testing.T) {
			events := []string{}
			p := &abandonTestProvider{events: &events, artifact: port.IssueProviderArtifactBody{State: tc.state}}
			record := model.IssueOpsRecord{Repo: "/repo", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Kind: tc.kind, URL: "u"}}
			// a caller-supplied flag is never authority
			req, err := ObserveAbandonArtifact(context.Background(), record, model.CleanupAbandonRequest{ArtifactUnmerged: !tc.wantUnmerged}, p)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err=%v", err)
				}
				return
			}
			if err != nil || req.ArtifactUnmerged != tc.wantUnmerged || p.gotKind != tc.wantKind {
				t.Fatalf("req=%+v err=%v kind=%q", req, err, p.gotKind)
			}
		})
	}
}

func TestObserveAbandonArtifactFailureModes(t *testing.T) {
	events := []string{}
	record := model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Kind: "pr", URL: "u"}}
	req, err := ObserveAbandonArtifact(context.Background(), model.IssueOpsRecord{}, model.CleanupAbandonRequest{ArtifactUnmerged: true}, nil)
	if err != nil || req.ArtifactUnmerged {
		t.Fatalf("no artifact must clear the flag: %+v %v", req, err)
	}
	if _, err := ObserveAbandonArtifact(context.Background(), record, model.CleanupAbandonRequest{}, struct{ port.IssueProvider }{}); err == nil || !strings.Contains(err.Error(), "does not support reading artifact state") {
		t.Fatalf("err=%v", err)
	}
	p := &abandonTestProvider{events: &events, artifactErr: errors.New("404")}
	if _, err := ObserveAbandonArtifact(context.Background(), record, model.CleanupAbandonRequest{}, p); err == nil || !strings.Contains(err.Error(), "abandon artifact readback failed: 404") {
		t.Fatalf("err=%v", err)
	}
}

func TestCleanupDrainerDrainsOnceAndKeepsOutcome(t *testing.T) {
	events := []string{}
	lease := &finishTestLease{events: &events, drainErr: errors.New("busy")}
	d := &cleanupDrainer{lifetime: lease}
	first, second := d.drain(context.Background()), d.drain(context.Background())
	if first == nil || first != second || d.drained {
		t.Fatalf("first=%v second=%v drained=%v", first, second, d.drained)
	}
	if n := len(events); n != 1 {
		t.Fatalf("drain attempts=%d events=%v", n, events)
	}
	d.close() // nil lifetime after a failed drain must be safe
}

func contains(list []string, want string) bool {
	return slices.Contains(list, want)
}
