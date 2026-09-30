package issueops

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestAbandonRefusesForeignAttemptBeforeAnyObservation(t *testing.T) {
	for _, operation := range []model.CleanupOperation{model.CleanupOperationFinish, model.CleanupOperationRemoteBranch} {
		t.Run(string(operation), func(t *testing.T) {
			root, record := remoteAbandonRecord(t)
			record.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: operation, Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
			_, raw, err := encodeIssueOpsRecord(record)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
				t.Fatal(err)
			}
			observations := 0
			executor := abandonExecutorForTests(root, CleanupAbandonDeps{Git: func(string, ...string) (int, string) { observations++; return 0, "" }})
			executor.Provider = func(string) (port.IssueProvider, error) { observations++; return nil, nil }
			executor.Observe = func(context.Context, model.IssueOpsRecord, model.CleanupAbandonRequest, port.IssueProvider) (model.CleanupAbandonRequest, error) {
				observations++
				return model.CleanupAbandonRequest{}, nil
			}
			if _, err := executor.Run(context.Background(), abandonRequest(record.ID, false, "")); err == nil {
				t.Fatal("foreign ownership accepted")
			}
			if observations != 0 {
				t.Fatalf("observed foreign-owned cycle: %d", observations)
			}
			current, _, err := db.Get(issueOpsBucket, record.ID)
			if err != nil || !bytes.Equal(current, raw) {
				t.Fatalf("record changed: %v", err)
			}
		})
	}
}

func TestAbandonArtifactDriftCannotArmOldEvidence(t *testing.T) {
	root, record := remoteAbandonRecord(t)
	remote := &fakeAbandonRemote{artifactBody: port.IssueProviderArtifactBody{State: "OPEN"}}
	executor := abandonExecutorForTests(root, remoteAbandonDeps(&remoteAbandonGit{}, remote))
	executor.Observe = cleanupapp.ObserveAbandonArtifact
	req := abandonRequest(record.ID, false, "")
	preview, err := executor.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	observer := executor.Observe
	executor.Observe = func(ctx context.Context, r model.IssueOpsRecord, request model.CleanupAbandonRequest, p port.IssueProvider) (model.CleanupAbandonRequest, error) {
		if r.RemoteArtifact.URL != record.RemoteArtifact.URL {
			t.Fatal("observer did not receive loaded artifact")
		}
		observed, err := observer(ctx, r, request, p)
		mutateFinishRecord(t, root, record.ID, func(current *model.IssueOpsRecord) {
			current.RemoteArtifact.URL = "https://github.com/acme/repo/pull/999"
		})
		return observed, err
	}
	executor.Git = func(context.Context, string, ...string) (int, string) { calls++; return 0, "" }
	req.Apply = true
	req.Confirm = true
	req.Fingerprint = preview.Fingerprint
	if _, err := executor.Run(context.Background(), req); err == nil {
		t.Fatal("artifact replacement was deleted using old observation")
	}
	current, err := ReadIssueOps(root, record.ID)
	if err != nil || current.RemoteArtifact.URL != "https://github.com/acme/repo/pull/999" || current.CleanupAttempt != nil || calls != 0 {
		t.Fatalf("replacement changed or effects executed: calls=%d current=%+v err=%v", calls, current, err)
	}
}

func TestAbandonCancellationPreservesFailureAndReleasesDrainedOwner(t *testing.T) {
	root, record := abandonTestRecord(t)
	executor := abandonExecutorForTests(root, abandonDeps(&fakeAbandonGit{branchOID: "abc123"}, authoritativeZeroOrca()))
	// A record-backed absent workspace authorizes the branch-only target.
	mutateFinishRecord(t, root, record.ID, func(r *model.IssueOpsRecord) {
		r.WorktreePath = filepath.Join(t.TempDir(), "absent-abandon-cancellation")
	})
	preview, err := executor.Run(context.Background(), abandonRequest(record.ID, false, ""))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	executor.Git = func(ctx context.Context, _ string, args ...string) (int, string) {
		calls++
		cancel()
		return 1, ctx.Err().Error()
	}
	got, err := executor.Run(ctx, abandonRequest(record.ID, true, preview.Fingerprint))
	if err == nil || got.RecordDeleted || got.FailedStep != model.CleanupFailureStepBranchDelete {
		t.Fatalf("cancelled mutation finalized: %+v %v", got, err)
	}
	kept, err := ReadIssueOps(root, record.ID)
	if err != nil || kept.CleanupAttempt != nil || kept.CleanupAbandonFailure == nil || !strings.Contains(kept.CleanupAbandonFailure.Message, context.Canceled.Error()) {
		t.Fatalf("failure/drain receipt missing: %+v %v", kept, err)
	}
	if calls != 1 {
		t.Fatalf("cancelled mutation must not start another command, got %d", calls)
	}
	// Already-cancelled context cannot acquire an execution lifetime.
	if _, err := executor.Run(ctx, abandonRequest(record.ID, false, "")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled preview: %v", err)
	}
}

type abandonReplacingProvider struct {
	*fakeAbandonRemote
	replace func()
}

func (p *abandonReplacingProvider) ClosePullRequest(ctx context.Context, req port.IssueProviderClosePullRequestRequest) (port.IssueProviderClosePullRequestResult, error) {
	result, err := p.fakeAbandonRemote.ClosePullRequest(ctx, req)
	p.replace()
	return result, err
}

func TestAbandonRemoteEffectDriftStopsRemainingEffectsAndPreservesReplacement(t *testing.T) {
	root, record := remoteAbandonRecord(t)
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	var replacement []byte
	remote := &abandonReplacingProvider{fakeAbandonRemote: &fakeAbandonRemote{
		artifactBody: port.IssueProviderArtifactBody{State: "OPEN"}, issueBody: port.IssueProviderArtifactBody{State: "OPEN"},
		closePR: port.IssueProviderClosePullRequestResult{OK: true, Closed: true, State: "CLOSED"},
	}}
	remote.replace = func() {
		current, err := ReadIssueOps(root, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		current.Branch = "replacement-branch"
		_, replacement, err = encodeIssueOpsRecord(current)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put(issueOpsBucket, record.ID, replacement); err != nil {
			t.Fatal(err)
		}
	}
	git := &remoteAbandonGit{remoteOID: strings.Repeat("b", 40)}
	executor := abandonExecutorForTests(root, remoteAbandonDeps(git, remote))
	preview, err := executor.Run(context.Background(), remoteAbandonRequest(record.ID, false, ""))
	if err != nil {
		t.Fatal(err)
	}
	got, err := executor.Run(context.Background(), remoteAbandonRequest(record.ID, true, preview.Fingerprint))
	if err == nil || got.FailedStep != model.CleanupFailureStepCloseIssue || got.RecordDeleted || strings.Join(got.RemoteEffects, ",") != "close_pr" {
		t.Fatalf("stale owner continued: %+v %v", got, err)
	}
	for _, call := range remote.calls {
		if strings.HasPrefix(call, "close_issue") {
			t.Fatal("issue closed after replacement")
		}
	}
	for _, command := range git.commands {
		if strings.HasPrefix(command, "push ") {
			t.Fatal("remote branch deleted after replacement")
		}
	}
	raw, _, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !bytes.Equal(raw, replacement) {
		t.Fatalf("failure receipt overwrote replacement: %v", err)
	}
}

func TestAbandonCancellationAfterLocalDeleteRecoversFromFreshInventory(t *testing.T) {
	root, record, _ := abandonResidueFixture(t)
	executor := abandonExecutorForTests(root, CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	preview, err := executor.Run(context.Background(), abandonRequest(record.ID, false, ""))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := executor.Git
	removed := false
	executor.Git = func(ctx context.Context, dir string, args ...string) (int, string) {
		code, out := command(ctx, dir, args...)
		if args[0] == "update-ref" && code == 0 {
			removed = true
			cancel()
			return 1, ctx.Err().Error()
		}
		return code, out
	}
	got, err := executor.Run(ctx, abandonRequest(record.ID, true, preview.Fingerprint))
	if !removed || !errors.Is(err, context.Canceled) || got.RecordDeleted {
		t.Fatalf("cancelled local deletion finalized: removed=%t %+v %v", removed, got, err)
	}
	kept, err := ReadIssueOps(root, record.ID)
	if err != nil || kept.CleanupAttempt != nil || kept.CleanupAbandonFailure == nil || kept.CleanupAbandonFailure.Step != model.CleanupFailureStepApplying {
		t.Fatalf("ambiguous local outcome lost: %+v %v", kept, err)
	}
	executor.Git = command
	preview, err = executor.Run(context.Background(), abandonRequest(record.ID, false, ""))
	if err != nil || preview.WorktreePresent || preview.BranchPresent {
		t.Fatalf("fresh absent inventory did not recover: %+v %v", preview, err)
	}
	got, err = executor.Run(context.Background(), abandonRequest(record.ID, true, preview.Fingerprint))
	if err != nil || !got.RecordDeleted {
		t.Fatalf("recovery did not finish: %+v %v", got, err)
	}
}
