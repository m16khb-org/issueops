package issueopscleanup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

type abandonPreviewEnv struct {
	directory      bool
	directoryErr   error
	sameLinked     bool
	local          AbandonWorktreeObservation
	branchOID      string
	branchOIDErr   error
	checkoutPath   string
	checkoutErr    error
	branchOIDCalls int
}

func (e *abandonPreviewEnv) Directory(string) (bool, error) { return e.directory, e.directoryErr }
func (e *abandonPreviewEnv) SamePath(a, b string) bool      { return a == b }
func (e *abandonPreviewEnv) SameLinkedPath(string, string) bool {
	return e.sameLinked
}
func (e *abandonPreviewEnv) Worktree(context.Context, string) AbandonWorktreeObservation {
	return e.local
}
func (e *abandonPreviewEnv) BranchOID(context.Context, string, string) (string, error) {
	e.branchOIDCalls++
	return e.branchOID, e.branchOIDErr
}
func (e *abandonPreviewEnv) BranchCheckoutPath(context.Context, string, string) (string, error) {
	return e.checkoutPath, e.checkoutErr
}

func healthyAbandonEnv() *abandonPreviewEnv {
	return &abandonPreviewEnv{
		directory: true, sameLinked: true, branchOID: "h1",
		local: AbandonWorktreeObservation{Canonical: true, Clean: true, HeadObservable: true, Branch: "feature", Head: "h1"},
	}
}

func localAbandonRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "cycle", Repo: "/repo", Branch: "feature", WorktreePath: "/wt"}
}

func abandonPreviewer(env *abandonPreviewEnv) AbandonPreviewer {
	return AbandonPreviewer{
		Environment: env,
		ReadChild:   func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, errors.New("not found") },
		Workspace: func(context.Context, model.IssueOpsRecord, string) (FinishWorkspaceObservation, []string) {
			return FinishWorkspaceObservation{}, nil
		},
	}
}

var abandonReason = model.CleanupAbandonRequest{Reason: "superseded by another approach"}

func TestAbandonPreviewerAcceptsHealthyLocalResidueAndCopiesObservations(t *testing.T) {
	env := healthyAbandonEnv()
	previewer := abandonPreviewer(env)
	previewer.Workspace = func(_ context.Context, _ model.IssueOpsRecord, root string) (FinishWorkspaceObservation, []string) {
		if root != "/wt" {
			t.Errorf("workspace observed %q", root)
		}
		return FinishWorkspaceObservation{
			Receipts:  []model.NativeProcessReceipt{{PID: 7, StartedAt: "t", Executable: "/bin/sh"}},
			Occupants: []model.CleanupWorkspaceProcess{{PID: 7, Command: "sh"}},
		}, nil
	}
	inventory, result := previewer.Plan(t.Context(), localAbandonRecord(), abandonReason, nil)
	if !result.OK || len(result.Missing) != 0 || !result.Preview {
		t.Fatalf("result=%+v", result)
	}
	if inventory.WorktreeRoot != "/wt" || !inventory.WorktreePresent || !inventory.WorktreeCanonical || !inventory.WorktreeClean || inventory.WorktreeHead != "h1" || inventory.WorktreeBranch != "feature" {
		t.Fatalf("worktree facts not copied: %+v", inventory)
	}
	if inventory.BranchOID != "h1" || len(inventory.WorkspaceProcesses) != 1 {
		t.Fatalf("branch/workspace facts not copied: %+v", inventory)
	}
	if inventory.RecordSHA != CleanupAbandonRecordSHA(localAbandonRecord()) || len(inventory.RecordSHA) != 64 {
		t.Fatalf("record seal missing: %q", inventory.RecordSHA)
	}
	if len(result.WorkspaceProcesses) != 1 || result.WorkspaceProcesses[0].PID != 7 {
		t.Fatalf("occupants not surfaced: %+v", result.WorkspaceProcesses)
	}
}

func TestAbandonPreviewerSkipsObservationsOfAbsentResources(t *testing.T) {
	env := healthyAbandonEnv()
	record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo"}
	inventory, result := abandonPreviewer(env).Plan(t.Context(), record, abandonReason, nil)
	if env.branchOIDCalls != 0 || inventory.WorktreePresent || inventory.BranchOID != "" {
		t.Fatalf("observed resources the record never named: %+v calls=%d", inventory, env.branchOIDCalls)
	}
	if !result.OK {
		t.Fatalf("result=%+v", result)
	}
}

func TestAbandonPreviewerReportsEachFailedObservationAsItsOwnGate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*abandonPreviewEnv, *model.IssueOpsRecord, *AbandonPreviewer)
		want   string
	}{
		{name: "linked path conflicts with execution root", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) { e.sameLinked = false }, want: "worktree_identity_conflict"},
		{name: "directory unobservable", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) {
			e.directory, e.directoryErr = false, errors.New("EACCES")
		}, want: "worktree_observable"},
		{name: "worktree not canonical", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) { e.local.Canonical = false }, want: "worktree_canonical"},
		{name: "worktree dirty", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) { e.local.Clean = false }, want: "worktree_clean"},
		{name: "head unobservable", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) {
			e.local.HeadObservable = false
		}, want: "worktree_head"},
		{name: "worktree on another branch", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) { e.local.Branch = "other" }, want: "worktree_branch_match"},
		{name: "worktree and branch heads differ", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) { e.branchOID = "h2" }, want: "local_branch_head"},
		{name: "branch ref unreadable", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) {
			e.branchOID, e.branchOIDErr = "", errors.New("git failed")
		}, want: "local_branch_observable"},
		{name: "registry unreadable", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) {
			e.checkoutErr = errors.New("worktree list failed")
		}, want: "worktree_registry_observable"},
		{name: "branch checked out elsewhere", mutate: func(e *abandonPreviewEnv, _ *model.IssueOpsRecord, _ *AbandonPreviewer) { e.checkoutPath = "/other" }, want: "branch_checked_out_elsewhere"},
		{name: "workspace gate passthrough", mutate: func(_ *abandonPreviewEnv, _ *model.IssueOpsRecord, p *AbandonPreviewer) {
			p.Workspace = func(context.Context, model.IssueOpsRecord, string) (FinishWorkspaceObservation, []string) {
				return FinishWorkspaceObservation{}, []string{"requester_occupies_worktree"}
			}
		}, want: "requester_occupies_worktree"},
		{name: "failure receipt does not match the inventory", mutate: func(_ *abandonPreviewEnv, r *model.IssueOpsRecord, _ *AbandonPreviewer) {
			r.CleanupAbandonFailure = &model.IssueOpsCleanupAbandonFailure{Step: "worktree_remove", WorktreePath: "/wt", InventorySHA256: "forged"}
		}, want: "cleanup_failure_inventory"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, record := healthyAbandonEnv(), localAbandonRecord()
			previewer := abandonPreviewer(env)
			tc.mutate(env, &record, &previewer)
			_, result := previewer.Plan(t.Context(), record, abandonReason, nil)
			if result.OK || !slices.Contains(result.Missing, tc.want) {
				t.Fatalf("missing=%v, want %q", result.Missing, tc.want)
			}
		})
	}
}

func TestAbandonPreviewerRequiresAReason(t *testing.T) {
	_, result := abandonPreviewer(healthyAbandonEnv()).Plan(t.Context(), localAbandonRecord(), model.CleanupAbandonRequest{}, nil)
	if result.OK || !slices.Contains(result.Missing, "reason_required") || result.ReasonError == "" {
		t.Fatalf("result=%+v", result)
	}
}

func TestAbandonPreviewerGatesOnLeaseChildrenAndArtifact(t *testing.T) {
	t.Run("live lease blocks abandon", func(t *testing.T) {
		record := localAbandonRecord()
		record.Execution = &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusActive}, Workspace: model.Workspace{Root: "/wt", Branch: "feature"}}
		_, result := abandonPreviewer(healthyAbandonEnv()).Plan(t.Context(), record, abandonReason, nil)
		if !slices.Contains(result.Missing, "lease_terminal") {
			t.Fatalf("missing=%v", result.Missing)
		}
	})
	t.Run("released lease passes", func(t *testing.T) {
		record := localAbandonRecord()
		record.Execution = &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusReleased}, Workspace: model.Workspace{Root: "/wt", Branch: "feature"}}
		_, result := abandonPreviewer(healthyAbandonEnv()).Plan(t.Context(), record, abandonReason, nil)
		if slices.Contains(result.Missing, "lease_terminal") {
			t.Fatalf("missing=%v", result.Missing)
		}
	})
	t.Run("remote artifact needs unmerged evidence", func(t *testing.T) {
		record := localAbandonRecord()
		record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{URL: "https://example.test/pull/1"}
		_, result := abandonPreviewer(healthyAbandonEnv()).Plan(t.Context(), record, abandonReason, nil)
		if !slices.Contains(result.Missing, "remote_artifact_unmerged") {
			t.Fatalf("missing=%v", result.Missing)
		}
		req := abandonReason
		req.ArtifactUnmerged = true
		_, result = abandonPreviewer(healthyAbandonEnv()).Plan(t.Context(), record, req, nil)
		if slices.Contains(result.Missing, "remote_artifact_unmerged") {
			t.Fatalf("missing=%v", result.Missing)
		}
	})
}

func TestAbandonPreviewerResolvesChildrenOnlyFromObservedDoneRecords(t *testing.T) {
	record := localAbandonRecord()
	record.ChildCycles = []model.IssueOpsChildCycleRef{{CycleID: " done "}, {CycleID: "open"}, {CycleID: "unreadable"}, {CycleID: "  "}}
	var read []string
	previewer := abandonPreviewer(healthyAbandonEnv())
	previewer.ReadChild = func(id string) (model.IssueOpsRecord, error) {
		read = append(read, id)
		switch id {
		case "done":
			return model.IssueOpsRecord{ID: "done", Phase: model.IssueOpsPhaseDone}, nil
		case "open":
			return model.IssueOpsRecord{ID: "open", Phase: model.IssueOpsPhaseImplement}, nil
		}
		return model.IssueOpsRecord{}, errors.New("missing")
	}
	resolved := previewer.ResolvedChildren(record)
	if !resolved["done"] || resolved["open"] || resolved["unreadable"] || !slices.Equal(read, []string{"done", "open", "unreadable"}) {
		t.Fatalf("resolved=%v read=%v", resolved, read)
	}
	_, result := previewer.Plan(t.Context(), record, abandonReason, nil)
	if !slices.Contains(result.Missing, "no_children") || !slices.Equal(result.UnresolvedChildren, []string{"open", "unreadable"}) {
		t.Fatalf("missing=%v unresolved=%v", result.Missing, result.UnresolvedChildren)
	}
}

func TestAbandonPreviewerSurfacesPendingIntent(t *testing.T) {
	record := localAbandonRecord()
	record.Execution = &model.Execution{
		Workspace: model.Workspace{Root: "/wt", Branch: "feature"},
		Lease:     model.WriteLease{Status: model.LeaseStatusReleased},
		Pending:   &model.ExternalIntent{OperationID: "op-9", Kind: "remote_pr_create"},
	}
	inventory, result := abandonPreviewer(healthyAbandonEnv()).Plan(t.Context(), record, abandonReason, nil)
	if inventory.PendingOperationID != "op-9" {
		t.Fatalf("pending operation lost: %+v", inventory)
	}
	if !slices.Contains(result.Missing, "pending_intent_safe") {
		t.Fatalf("missing=%v, want pending_intent_safe", result.Missing)
	}
	if !strings.Contains(result.PendingIntentError, "issueops execution reconcile --id cycle") || !strings.Contains(result.PendingIntentError, "remote_pr_create") {
		t.Fatalf("pending=%q", result.PendingIntentError)
	}
}

func TestAbandonPreviewerFeedsRemoteEffectsIntoThePreview(t *testing.T) {
	record := localAbandonRecord()
	record.IssueURL = "https://example.test/issues/1"
	previewer := abandonPreviewer(healthyAbandonEnv())
	var calls []remoteRefCall
	previewer.Remote = AbandonRemoteObserver{RemoteRef: remoteRefFake("remote-oid", nil, &calls)}
	reader := &recordingBodyReader{states: map[string]string{"https://example.test/issues/1": "OPEN"}}
	req := abandonReason
	req.CloseIssue, req.DeleteRemoteBranch = true, true
	inventory, result := previewer.Plan(t.Context(), record, req, reader)
	if !slices.Equal(result.RemoteEffects, []string{"close_issue", "remote_branch_delete"}) || inventory.RemoteBranchOID != "remote-oid" || result.IssueState != "OPEN" {
		t.Fatalf("inventory=%+v result=%+v", inventory, result)
	}
}

func TestAbandonPreviewerTrustsOnlyAuthenticallySealedFailureReceipts(t *testing.T) {
	hex64 := func(c string) string { return strings.Repeat(c, 64) }
	record := localAbandonRecord()
	sealed := abandonFailure(record, model.CleanupAbandonInventory{
		RecordSHA: hex64("a"), WorktreeRoot: "/wt", Branch: "feature", WorktreeHead: "h1", BranchOID: "h1",
	}, hex64("b"), "worktree_remove", "git failed", "2026-09-29T00:00:00Z")
	tampered := sealed
	tampered.RecordSHA = hex64("c") // still well-formed, but no longer what the seal covers
	for _, tc := range []struct {
		name    string
		failure model.IssueOpsCleanupAbandonFailure
		trusted bool
	}{
		{"sealed receipt matching the live resources", sealed, true},
		{"well-formed receipt whose sealed fields were altered", tampered, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := record
			failure := tc.failure
			current.CleanupAbandonFailure = &failure
			_, result := abandonPreviewer(healthyAbandonEnv()).Plan(t.Context(), current, abandonReason, nil)
			if got := !slices.Contains(result.Missing, "cleanup_failure_inventory"); got != tc.trusted {
				t.Fatalf("trusted=%v want %v missing=%v", got, tc.trusted, result.Missing)
			}
		})
	}
}
