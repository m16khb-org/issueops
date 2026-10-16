package issueopsreplacement

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

type failingVerifier struct{ err error }

func (v failingVerifier) Verify(context.Context, model.NativeActor) (model.VerifiedActor, error) {
	return model.VerifiedActor{}, v.err
}

type loadFailsRecords struct {
	*replacementFixture
	err error
}

func (r loadFailsRecords) Load(string) (model.IssueOpsRecord, error) {
	return model.IssueOpsRecord{}, r.err
}

type statusProcesses struct {
	deadProcesses
	status string
	err    error
}

func (p statusProcesses) Inspect(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	return p.status, receipt, p.err
}

type errWorkspace struct {
	*replacementFixture
	snapshotErr  error
	processesErr error
}

func (w errWorkspace) WorkspaceSnapshot(ws model.Workspace) (string, error) {
	if w.snapshotErr != nil {
		return "", w.snapshotErr
	}
	return w.replacementFixture.WorkspaceSnapshot(ws)
}

func (w errWorkspace) WorkspaceProcesses(root string, excluded map[int]bool) ([]model.ReplacementWorkspaceProcess, error) {
	if w.processesErr != nil {
		return nil, w.processesErr
	}
	return w.replacementFixture.WorkspaceProcesses(root, excluded)
}

type snapshotWorkspace struct {
	*replacementFixture
	snapshot string
}

func (w snapshotWorkspace) WorkspaceSnapshot(model.Workspace) (string, error) { return w.snapshot, nil }

type artifactStub struct {
	*replacementFixture
	cleanupErrs []error // consumed per Cleanup call; exhausted → nil
	cleanups    int
	tokenErr    error
}

func (a *artifactStub) Cleanup(record model.IssueOpsRecord) error {
	a.cleanups++
	if len(a.cleanupErrs) == 0 {
		return nil
	}
	err := a.cleanupErrs[0]
	a.cleanupErrs = a.cleanupErrs[1:]
	return err
}

func (a *artifactStub) CreateToken(record model.IssueOpsRecord) (string, string, error) {
	if a.tokenErr != nil {
		return "", "", a.tokenErr
	}
	return a.replacementFixture.CreateToken(record)
}

type baseSyncStub struct {
	receipt  basesyncport.Receipt
	err      error
	requests []basesyncport.Request
}

func (b *baseSyncStub) Observe(_ context.Context, req basesyncport.Request) (basesyncport.Receipt, error) {
	b.requests = append(b.requests, req)
	return b.receipt, b.err
}

func TestRunRefusesWithoutVerifier(t *testing.T) {
	service, _, req := fixtureService()
	service.Verifier = nil
	result, err := service.Run(t.Context(), req)
	if err == nil || !strings.Contains(err.Error(), "actor verifier is required") || result.OK || result.ID != "io-test" || result.Action != req.Action {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestRunSurfacesActorVerificationFailureBeforeLoadingRecord(t *testing.T) {
	service, f, req := fixtureService()
	boom := errors.New("actor not live")
	service.Verifier = failingVerifier{err: boom}
	result, err := service.Run(t.Context(), req)
	if !errors.Is(err, boom) || result.OK || len(f.events) != 0 {
		t.Fatalf("result=%+v err=%v events=%v", result, err, f.events)
	}
}

func TestRunRejectsUnsupportedAction(t *testing.T) {
	service, f, req := fixtureService()
	req.Action = model.ExecutionReplaceReseed
	result, err := service.Run(t.Context(), req)
	if err == nil || !strings.Contains(err.Error(), `unsupported execution replace action "reseed"`) || result.OK || len(f.events) != 0 {
		t.Fatalf("result=%+v err=%v events=%v", result, err, f.events)
	}
}

func TestPreviewRefusalsReportNotOKAndNeverPersist(t *testing.T) {
	boom := errors.New("snapshot unavailable")
	for _, tc := range []struct {
		name   string
		change func(*Service, *replacementFixture, *model.ExecutionReplaceRequest)
		want   string
	}{
		{"load failure", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			s.Records = loadFailsRecords{f, boom}
		}, "snapshot unavailable"},
		{"stale generation", func(_ *Service, _ *replacementFixture, r *model.ExecutionReplaceRequest) { r.ExpectedGeneration = 3 }, "stale lease generation: current=7 expected=3"},
		{"revoking lease", func(_ *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
		}, "replace preview is unavailable from revoking"},
		{"cwd outside workspace", func(_ *Service, _ *replacementFixture, r *model.ExecutionReplaceRequest) { r.CWD = "/elsewhere" }, "cwd must be source_root"},
		{"workspace snapshot failure", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			s.Workspace = errWorkspace{replacementFixture: f, snapshotErr: boom}
		}, "snapshot unavailable"},
		{"holder process inspection failure", func(s *Service, _ *replacementFixture, _ *model.ExecutionReplaceRequest) {
			s.ObserveProcesses = func() port.ReplacementProcessSnapshot { return statusProcesses{err: boom} }
		}, "snapshot unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, f, req := fixtureService()
			tc.change(&service, f, &req)
			result, err := service.Run(t.Context(), req)
			if err == nil || !strings.Contains(err.Error(), tc.want) || result.OK || result.InventoryFingerprint != "" || f.saved != nil {
				t.Fatalf("result=%+v err=%v saved=%+v", result, err, f.saved)
			}
		})
	}
}

func TestPreviewOfActiveLeaseOffersRevokeCommandWithFingerprint(t *testing.T) {
	service, _, req := fixtureService()
	result, err := service.Run(t.Context(), req)
	if err != nil || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, want := range []string{"--revoke", "--expected-generation 7", "--inventory-fingerprint " + result.InventoryFingerprint, "--session-id 'requester'", "--cwd '/workspace'"} {
		if !strings.Contains(result.NextCommand, want) {
			t.Fatalf("NextCommand %q lacks %q", result.NextCommand, want)
		}
	}
	if result.QuiescenceFingerprint != "" || result.ClaimTokenPath != "" {
		t.Fatalf("preview leaked finalize fields: %+v", result)
	}
}

func TestPreviewWithdrawsRevokeCommandWhenRequesterIsLiveHolder(t *testing.T) {
	service, f, req := fixtureService()
	f.record.Execution.Lease.Holder = &model.NativeActor{Host: req.Actor.Host, SessionID: req.Actor.SessionID, SessionProcess: req.Actor.SessionProcess}
	result, err := service.Run(t.Context(), req)
	if err != nil || !result.OK || result.NextCommand != "" {
		t.Fatalf("a live holder must not be steered into revoking itself: result=%+v err=%v", result, err)
	}
}

func TestPreviewKeepsRevokeCommandWhenSelfHolderIsDeadOrUninspectable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inspect func(model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)
	}{
		{"dead", func(p model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) { return "dead", p, nil }},
		{"inspection error", func(p model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
			return "", p, errors.New("ps failed")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, f, req := fixtureService()
			f.record.Execution.Lease.Holder = &model.NativeActor{Host: req.Actor.Host, SessionID: req.Actor.SessionID, SessionProcess: req.Actor.SessionProcess}
			service.InspectProcess = tc.inspect
			result, err := service.Run(t.Context(), req)
			if err != nil || !strings.Contains(result.NextCommand, "--revoke") {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestRevokeRefusesToTakeLeaseFromLiveRequesterSession(t *testing.T) {
	service, f, req := fixtureService()
	f.record.Execution.Lease.Holder = &model.NativeActor{Host: req.Actor.Host, SessionID: req.Actor.SessionID, SessionProcess: req.Actor.SessionProcess}
	req.Action = model.ExecutionReplaceRevoke
	result, err := service.Run(t.Context(), req)
	if err == nil || !strings.Contains(err.Error(), "execution release --id io-test --generation 7") || result.OK || f.saved != nil {
		t.Fatalf("result=%+v err=%v saved=%+v", result, err, f.saved)
	}
}

func TestPreviewOfReleasedLeaseOffersReseedCommand(t *testing.T) {
	service, f, req := fixtureService()
	f.record.Execution.Lease.Status = model.LeaseStatusReleased
	f.record.Execution.Lease.Holder = nil
	result, err := service.Run(t.Context(), req)
	if err != nil || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, want := range []string{"--reseed", "--inventory-fingerprint " + result.InventoryFingerprint, "--session-pid 42"} {
		if !strings.Contains(result.NextCommand, want) {
			t.Fatalf("NextCommand %q lacks %q", result.NextCommand, want)
		}
	}
	if strings.Contains(result.NextCommand, "--completion-generation") {
		t.Fatalf("no completion generation was selected: %q", result.NextCommand)
	}
}

func TestInventoryFingerprintTracksRequesterAndWorkspaceSnapshot(t *testing.T) {
	service, _, req := fixtureService()
	first, err := service.Run(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.Run(t.Context(), req)
	if err != nil || again.InventoryFingerprint != first.InventoryFingerprint {
		t.Fatalf("fingerprint must be deterministic: %q vs %q err=%v", first.InventoryFingerprint, again.InventoryFingerprint, err)
	}
	other := req
	other.Actor.SessionID = "someone-else"
	changed, err := service.Run(t.Context(), other)
	if err != nil || changed.InventoryFingerprint == first.InventoryFingerprint {
		t.Fatalf("fingerprint must bind the requester: %q err=%v", changed.InventoryFingerprint, err)
	}
	service.Workspace = snapshotWorkspace{replacementFixture: service.Workspace.(*replacementFixture), snapshot: "other-snapshot"}
	moved, err := service.Run(t.Context(), req)
	if err != nil || moved.InventoryFingerprint == first.InventoryFingerprint {
		t.Fatalf("fingerprint must bind the workspace snapshot: %q err=%v", moved.InventoryFingerprint, err)
	}
}

func completedReleasedFixture() (Service, *replacementFixture, model.ExecutionReplaceRequest) {
	service, f, req := fixtureService()
	f.record.Execution.Lease.Status = model.LeaseStatusReleased
	f.record.Execution.Lease.Holder = nil
	f.record.Execution.Completion = &model.ExecutionCompletion{Generation: 3}
	f.record.BranchPrepare = &model.IssueOpsBranchPrepare{BaseBranch: "main"}
	return service, f, req
}

func TestCompletedPreviewRequiresBaseSyncInspector(t *testing.T) {
	service, _, req := completedReleasedFixture()
	_, err := service.Run(t.Context(), req)
	if err == nil || !strings.Contains(err.Error(), "requires base sync inspector") {
		t.Fatalf("err=%v", err)
	}
}

func TestCompletedPreviewRejectsInvalidCompletionSelections(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*replacementFixture, *model.ExecutionReplaceRequest)
		want   string
	}{
		{"unstamped completion", func(f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Completion.Generation = 0
		}, "invalid or missing stamped completion generation"},
		{"conflicting selection", func(_ *replacementFixture, r *model.ExecutionReplaceRequest) { r.CompletionGeneration = 2 }, "conflicts with stamped completion generation 3"},
		{"missing base branch", func(f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.BranchPrepare.BaseBranch = "  "
		}, "requires branch_prepare.base_branch"},
		{"missing branch prepare", func(f *replacementFixture, _ *model.ExecutionReplaceRequest) { f.record.BranchPrepare = nil }, "requires branch_prepare.base_branch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, f, req := completedReleasedFixture()
			sync := &baseSyncStub{}
			service.BaseSync = sync
			tc.change(f, &req)
			_, err := service.Run(t.Context(), req)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(sync.requests) != 0 {
				t.Fatalf("err=%v observed=%d", err, len(sync.requests))
			}
		})
	}
}

func TestCompletedPreviewWrapsBaseObservationFailure(t *testing.T) {
	service, _, req := completedReleasedFixture()
	boom := errors.New("fetch failed")
	service.BaseSync = &baseSyncStub{err: boom}
	_, err := service.Run(t.Context(), req)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "observe completed execution base") {
		t.Fatalf("err=%v", err)
	}
}

func TestCompletedPreviewDemandsBaseSyncWhenWorkBranchIsBehind(t *testing.T) {
	service, _, req := completedReleasedFixture()
	sync := &baseSyncStub{receipt: basesyncport.Receipt{SyncRequired: true}}
	service.BaseSync = sync
	result, err := service.Run(t.Context(), req)
	syncErr, ok := errors.AsType[*model.BaseSyncRequiredError](err)
	if !ok || syncErr.ID != "io-test" || syncErr.CompletionGeneration != 3 || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(sync.requests) != 1 || sync.requests[0].Worktree != "/workspace" || sync.requests[0].BaseBranch != "main" {
		t.Fatalf("observed %+v", sync.requests)
	}
}

func TestCompletedPreviewProceedsToReseedWhenBaseIsInSync(t *testing.T) {
	service, _, req := completedReleasedFixture()
	service.BaseSync = &baseSyncStub{}
	req.CompletionGeneration = 3
	result, err := service.Run(t.Context(), req)
	if err != nil || !result.OK || !strings.Contains(result.NextCommand, "--completion-generation 3") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestFinalizePreviewRefusals(t *testing.T) {
	boom := errors.New("ps unreadable")
	for _, tc := range []struct {
		name   string
		change func(*Service, *replacementFixture, *model.ExecutionReplaceRequest)
		want   string
	}{
		{"load failure", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			s.Records = loadFailsRecords{f, boom}
		}, "ps unreadable"},
		{"stale generation", func(_ *Service, _ *replacementFixture, r *model.ExecutionReplaceRequest) { r.ExpectedGeneration = 6 }, "stale lease generation"},
		{"discovery generation is not allowed", func(_ *Service, _ *replacementFixture, r *model.ExecutionReplaceRequest) { r.ExpectedGeneration = 0 }, "stale lease generation"},
		{"lease not revoking", func(_ *Service, _ *replacementFixture, _ *model.ExecutionReplaceRequest) {}, "finalize preview requires a revoking lease"},
		{"cwd outside workspace", func(_ *Service, f *replacementFixture, r *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			r.CWD = "/elsewhere"
		}, "cwd must be source_root"},
		{"holder without process receipt", func(_ *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			f.record.Execution.Lease.Holder.SessionProcess = nil
		}, "missing its old process receipt"},
		{"holder still live", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			s.ObserveProcesses = func() port.ReplacementProcessSnapshot { return statusProcesses{status: "live"} }
		}, "old holder process is still live: pid=41 executable=/old-host"},
		{"holder identity unsafe", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			s.ObserveProcesses = func() port.ReplacementProcessSnapshot { return statusProcesses{status: "reused"} }
		}, "unsafe to finalize: pid=41 status=reused"},
		{"holder inspection failure", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			s.ObserveProcesses = func() port.ReplacementProcessSnapshot { return statusProcesses{err: boom} }
		}, "ps unreadable"},
		{"workspace process listing failure", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			s.Workspace = errWorkspace{replacementFixture: f, processesErr: boom}
		}, "ps unreadable"},
		{"workspace snapshot failure", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			s.Workspace = errWorkspace{replacementFixture: f, snapshotErr: boom}
		}, "ps unreadable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, f, req := fixtureService()
			req.Action = model.ExecutionReplaceFinalizePreview
			tc.change(&service, f, &req)
			result, err := service.Run(t.Context(), req)
			if err == nil || !strings.Contains(err.Error(), tc.want) || result.OK || result.QuiescenceFingerprint != "" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestFinalizeRefusals(t *testing.T) {
	cleanupBoom := errors.New("cleanup denied")
	for _, tc := range []struct {
		name   string
		change func(*Service, *replacementFixture, *model.ExecutionReplaceRequest)
		want   string
	}{
		{"lease not revoking", func(_ *Service, _ *replacementFixture, _ *model.ExecutionReplaceRequest) {}, "finalize requires a revoking lease"},
		{"pending external intent", func(_ *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			f.record.Execution.Pending = &model.ExternalIntent{}
		}, "pending external intent"},
		{"stale generation", func(_ *Service, f *replacementFixture, r *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			r.ExpectedGeneration = 2
		}, "stale lease generation"},
		{"cwd outside workspace", func(_ *Service, f *replacementFixture, r *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			r.CWD = "/elsewhere"
		}, "cwd must be source_root"},
		{"holder still live", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			s.ObserveProcesses = func() port.ReplacementProcessSnapshot { return statusProcesses{status: "live"} }
		}, "old holder process is still live"},
		{"cleanup failure", func(s *Service, f *replacementFixture, _ *model.ExecutionReplaceRequest) {
			f.record.Execution.Lease.Status = model.LeaseStatusRevoking
			s.Artifacts = &artifactStub{replacementFixture: f, cleanupErrs: []error{cleanupBoom}}
		}, "cleanup denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, f, req := fixtureService()
			req.Action = model.ExecutionReplaceFinalize
			// Obtain a matching fingerprint so only the case's own defect can refuse.
			if tc.name != "lease not revoking" {
				probeSvc, probeFixture, probeReq := fixtureService()
				probeFixture.record.Execution.Lease.Status = model.LeaseStatusRevoking
				probeReq.Action = model.ExecutionReplaceFinalizePreview
				preview, err := probeSvc.Run(t.Context(), probeReq)
				if err != nil {
					t.Fatal(err)
				}
				req.QuiescenceFingerprint = preview.QuiescenceFingerprint
			}
			tc.change(&service, f, &req)
			result, err := service.Run(t.Context(), req)
			if err == nil || !strings.Contains(err.Error(), tc.want) || result.OK || f.saved != nil {
				t.Fatalf("result=%+v err=%v saved=%+v", result, err, f.saved)
			}
		})
	}
}

func finalizeRequest(t *testing.T, service Service) model.ExecutionReplaceRequest {
	t.Helper()
	_, _, req := fixtureService()
	req.Action = model.ExecutionReplaceFinalizePreview
	preview, err := service.Run(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	req.Action = model.ExecutionReplaceFinalize
	req.QuiescenceFingerprint = preview.QuiescenceFingerprint
	return req
}

func TestFinalizeCleanupFailureAfterTokenErrorReportsBothCauses(t *testing.T) {
	service, f, _ := fixtureService()
	f.record.Execution.Lease.Status = model.LeaseStatusRevoking
	req := finalizeRequest(t, service)
	// First Cleanup (pre-token) succeeds, second (post-failure residue) fails.
	artifacts := &artifactStub{replacementFixture: f, tokenErr: errors.New("token write failed"), cleanupErrs: []error{nil, errors.New("rm denied")}}
	service.Artifacts = artifacts
	result, err := service.Run(t.Context(), req)
	if err == nil || result.OK || f.saved != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, want := range []string{"token write failed", "replacement residue cleanup failed: rm denied"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err %q lacks %q", err, want)
		}
	}
	if artifacts.cleanups != 2 {
		t.Fatalf("cleanups=%d", artifacts.cleanups)
	}
}

func TestFinalizeCleanupFailureAfterPersistErrorKeepsOriginalCauseWrapped(t *testing.T) {
	service, f, _ := fixtureService()
	f.record.Execution.Lease.Status = model.LeaseStatusRevoking
	req := finalizeRequest(t, service)
	boom := errors.New("persist broke")
	f.saveError = boom
	service.Artifacts = &artifactStub{replacementFixture: f, cleanupErrs: []error{nil, errors.New("rm denied")}}
	_, err := service.Run(t.Context(), req)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "replacement residue cleanup failed: rm denied") {
		t.Fatalf("err=%v", err)
	}
}

func TestFinalizeOffersClaimCommandAndReportsTokenPath(t *testing.T) {
	service, f, _ := fixtureService()
	f.record.Execution.Lease.Status = model.LeaseStatusRevoking
	req := finalizeRequest(t, service)
	result, err := service.Run(t.Context(), req)
	if err != nil || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if lease := result.Execution.Lease; lease.Status != model.LeaseStatusClaimable || lease.ClaimTokenSHA256 != "digest" || lease.Holder != nil {
		t.Fatalf("lease=%+v", lease)
	}
	if want := "issueops execution claim --id 'io-test' --generation 7 --claim-current-token"; result.NextCommand != want {
		t.Fatalf("NextCommand=%q want %q", result.NextCommand, want)
	}
	if result.ClaimTokenPath != "/workspace/claim.token" {
		t.Fatalf("ClaimTokenPath=%q", result.ClaimTokenPath)
	}
}
