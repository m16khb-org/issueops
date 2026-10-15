package issueops

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

const covAbandonToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func covAbandonReceipt(step string) *model.IssueOpsCleanupAbandonFailure {
	return &model.IssueOpsCleanupAbandonFailure{
		Step: step, Branch: "task",
		Fingerprint: strings.Repeat("a", 64), RecordSHA: strings.Repeat("b", 64), InventorySHA256: strings.Repeat("c", 64),
	}
}

var covAbandonEvidence = CleanupAbandonFailureEvidence{SameWorktree: true, SealMatches: true}

func TestCleanupAbandonRemoteFlagsAndPreviewCommand(t *testing.T) {
	if got := CleanupAbandonRemoteFlags(model.CleanupAbandonRequest{}); got != "" {
		t.Fatalf("no flags expected, got %q", got)
	}
	req := model.CleanupAbandonRequest{ClosePR: true, CloseIssue: true, DeleteRemoteBranch: true}
	if got := CleanupAbandonRemoteFlags(req); got != " --close-pr --close-issue --delete-remote-branch" {
		t.Fatalf("flags: %q", got)
	}
	if got := CleanupAbandonRemoteFlags(model.CleanupAbandonRequest{CloseIssue: true}); got != " --close-issue" {
		t.Fatalf("single flag: %q", got)
	}
	got := CleanupAbandonPreviewCommand("io-1", "cancel it", model.CleanupAbandonRequest{ClosePR: true})
	want := `issueops cleanup abandon --id io-1 --reason "cancel it" --close-pr --preview --json`
	if got != want {
		t.Fatalf("command:\n got %s\nwant %s", got, want)
	}
}

func TestCleanupAbandonFailureInventoryMatchesNilReceiptAlwaysMatches(t *testing.T) {
	if !CleanupAbandonFailureInventoryMatches(model.IssueOpsRecord{}, model.CleanupAbandonInventory{Branch: "x"}, CleanupAbandonFailureEvidence{}) {
		t.Fatal("no receipt must not block")
	}
}

func TestCleanupAbandonFailureInventoryMatchesRejectsMalformedReceipt(t *testing.T) {
	inv := model.CleanupAbandonInventory{Branch: "task"}
	for name, mutate := range map[string]func(*model.IssueOpsCleanupAbandonFailure){
		"short fingerprint":   func(f *model.IssueOpsCleanupAbandonFailure) { f.Fingerprint = "abc" },
		"non-hex record sha":  func(f *model.IssueOpsCleanupAbandonFailure) { f.RecordSHA = strings.Repeat("z", 64) },
		"short inventory sha": func(f *model.IssueOpsCleanupAbandonFailure) { f.InventorySHA256 = "" },
		"other branch":        func(f *model.IssueOpsCleanupAbandonFailure) { f.Branch = "other" },
		"unknown step":        func(f *model.IssueOpsCleanupAbandonFailure) { f.Step = "teleport" },
		"paired oids differ": func(f *model.IssueOpsCleanupAbandonFailure) {
			f.WorktreeHead, f.BranchOID = "one", "two"
		},
	} {
		t.Run(name, func(t *testing.T) {
			failure := covAbandonReceipt("record_delete")
			mutate(failure)
			if CleanupAbandonFailureInventoryMatches(model.IssueOpsRecord{CleanupAbandonFailure: failure}, inv, covAbandonEvidence) {
				t.Fatal("malformed receipt accepted")
			}
		})
	}
}

func TestCleanupAbandonFailureInventoryMatchesByStep(t *testing.T) {
	type shape struct{ worktree, branch string } // "" means absent in that axis
	inv := func(s shape) model.CleanupAbandonInventory {
		return model.CleanupAbandonInventory{Branch: "task", WorktreePresent: s.worktree != "", WorktreeHead: s.worktree, BranchOID: s.branch}
	}
	cases := []struct {
		name     string
		step     string
		orig     shape
		now      shape
		wantFits bool
	}{
		{"applying: nothing removed yet", "applying", shape{"h", "h"}, shape{"h", "h"}, true},
		{"applying: worktree removed, branch kept", "applying", shape{"h", "h"}, shape{"", "h"}, true},
		{"applying: both removed", "applying", shape{"h", "h"}, shape{"", ""}, true},
		{"applying: branch gone but worktree still present", "applying", shape{"h", "h"}, shape{"h", ""}, false},
		{"applying: branch moved", "applying", shape{"h", "h"}, shape{"", "x"}, false},
		{"applying: worktree appeared that was never recorded", "applying", shape{"", "h"}, shape{"h", "h"}, false},
		{"worktree_remove: still present", "worktree_remove", shape{"h", "h"}, shape{"h", "h"}, true},
		{"worktree_remove: worktree already gone", "worktree_remove", shape{"h", "h"}, shape{"", "h"}, false},
		{"worktree_remove: branch-only origin has no worktree to remove", "worktree_remove", shape{"", "h"}, shape{"", "h"}, false},
		{"workspace_processes_stop: branch vanished", "workspace_processes_stop", shape{"h", "h"}, shape{"h", ""}, false},
		{"workspace_processes_stop: unchanged", "workspace_processes_stop", shape{"h", "h"}, shape{"h", "h"}, true},
		{"branch_delete: worktree gone, branch present", "branch_delete", shape{"h", "h"}, shape{"", "h"}, true},
		{"branch_delete: worktree reappeared", "branch_delete", shape{"h", "h"}, shape{"h", "h"}, false},
		{"branch_delete: branch already gone", "branch_delete", shape{"h", "h"}, shape{"", ""}, false},
		{"branch_delete: branch never recorded", "branch_delete", shape{"h", ""}, shape{"", "h"}, false},
		{"record_delete: everything removed", "record_delete", shape{"h", "h"}, shape{"", ""}, true},
		{"record_delete: branch remains", "record_delete", shape{"h", "h"}, shape{"", "h"}, false},
		{"record_delete: worktree remains", "record_delete", shape{"h", "h"}, shape{"h", ""}, false},
		{"record_delete: nothing ever existed", "record_delete", shape{"", ""}, shape{"", ""}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failure := covAbandonReceipt(tc.step)
			failure.WorktreeHead, failure.BranchOID = tc.orig.worktree, tc.orig.branch
			record := model.IssueOpsRecord{CleanupAbandonFailure: failure}
			if got := CleanupAbandonFailureInventoryMatches(record, inv(tc.now), covAbandonEvidence); got != tc.wantFits {
				t.Fatalf("got %t want %t", got, tc.wantFits)
			}
		})
	}
}

func TestCleanupAbandonPartialBranchRetry(t *testing.T) {
	base := func() (model.IssueOpsRecord, model.CleanupAbandonInventory) {
		failure := covAbandonReceipt("branch_delete")
		failure.WorktreeHead, failure.BranchOID = "h", "h"
		return model.IssueOpsRecord{CleanupAbandonFailure: failure}, model.CleanupAbandonInventory{Branch: "task", BranchOID: "h"}
	}
	record, inv := base()
	if !CleanupAbandonPartialBranchRetry(record, inv, covAbandonEvidence) {
		t.Fatal("branch-only residue after worktree removal must be retryable")
	}
	record.CleanupAbandonFailure.Step = "applying"
	if !CleanupAbandonPartialBranchRetry(record, inv, covAbandonEvidence) {
		t.Fatal("applying step must be retryable")
	}
	for name, mutate := range map[string]func(*model.IssueOpsRecord, *model.CleanupAbandonInventory, *CleanupAbandonFailureEvidence){
		"no receipt": func(r *model.IssueOpsRecord, _ *model.CleanupAbandonInventory, _ *CleanupAbandonFailureEvidence) {
			r.CleanupAbandonFailure = nil
		},
		"worktree present": func(_ *model.IssueOpsRecord, i *model.CleanupAbandonInventory, _ *CleanupAbandonFailureEvidence) {
			i.WorktreePresent = true
		},
		"branch absent": func(_ *model.IssueOpsRecord, i *model.CleanupAbandonInventory, _ *CleanupAbandonFailureEvidence) {
			i.BranchOID = ""
		},
		"other step": func(r *model.IssueOpsRecord, _ *model.CleanupAbandonInventory, _ *CleanupAbandonFailureEvidence) {
			r.CleanupAbandonFailure.Step = "record_delete"
		},
		"different worktree": func(_ *model.IssueOpsRecord, _ *model.CleanupAbandonInventory, e *CleanupAbandonFailureEvidence) {
			e.SameWorktree = false
		},
		"branch renamed": func(_ *model.IssueOpsRecord, i *model.CleanupAbandonInventory, _ *CleanupAbandonFailureEvidence) {
			i.Branch = "other"
		},
		"oid moved": func(_ *model.IssueOpsRecord, i *model.CleanupAbandonInventory, _ *CleanupAbandonFailureEvidence) {
			i.BranchOID = "m"
		},
		"recorded head differs": func(r *model.IssueOpsRecord, _ *model.CleanupAbandonInventory, _ *CleanupAbandonFailureEvidence) {
			r.CleanupAbandonFailure.WorktreeHead = "other"
		},
	} {
		t.Run(name, func(t *testing.T) {
			record, inv := base()
			ev := covAbandonEvidence
			mutate(&record, &inv, &ev)
			if CleanupAbandonPartialBranchRetry(record, inv, ev) {
				t.Fatal("must not be a partial branch retry")
			}
		})
	}
}

func TestCleanupAbandonRemovalPlanListsOnlyExistingResources(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-1", Execution: &model.Execution{
		Pending: &model.ExternalIntent{OperationID: " op-a "},
		Failure: &model.ExecutionFailure{OperationID: "op-a"},
	}}
	got := CleanupAbandonRemovalPlan(record, model.CleanupAbandonInventory{WorktreePresent: true, WorktreeRoot: "/w", Branch: "task", BranchOID: "abc"})
	want := []string{"local_worktree:/w", "local_branch:task@abc", "record:io-1", "intent:op-a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = CleanupAbandonRemovalPlan(model.IssueOpsRecord{ID: "io-2"}, model.CleanupAbandonInventory{Branch: "task"})
	if !reflect.DeepEqual(got, []string{"record:io-2"}) {
		t.Fatalf("record-only plan: %v", got)
	}
}

func TestCleanupAbandonIntentOperationIDsDedupesAndSkipsBlank(t *testing.T) {
	if got := CleanupAbandonIntentOperationIDs(model.IssueOpsRecord{}); got != nil {
		t.Fatalf("no execution: %v", got)
	}
	record := model.IssueOpsRecord{Execution: &model.Execution{Pending: &model.ExternalIntent{OperationID: "  "}, Failure: &model.ExecutionFailure{OperationID: "op-f"}}}
	if got := CleanupAbandonIntentOperationIDs(record); !reflect.DeepEqual(got, []string{"op-f"}) {
		t.Fatalf("blank pending skipped: %v", got)
	}
	record.Execution.Pending.OperationID = "op-p"
	if got := CleanupAbandonIntentOperationIDs(record); !reflect.DeepEqual(got, []string{"op-p", "op-f"}) {
		t.Fatalf("pending first then failure: %v", got)
	}
	record.Execution.Failure.OperationID = " op-p"
	if got := CleanupAbandonIntentOperationIDs(record); !reflect.DeepEqual(got, []string{"op-p"}) {
		t.Fatalf("trimmed duplicate collapsed: %v", got)
	}
}

func TestCleanupAbandonUnresolvedChildren(t *testing.T) {
	record := model.IssueOpsRecord{
		IssueURL: "https://x/issues/1",
		ChildCycles: []model.IssueOpsChildCycleRef{
			{CycleID: " c1 "}, {CycleID: "c2"}, {CycleID: "  "},
		},
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: " https://x/issues/2 "},
			{Type: "child", URL: "https://x/issues/3", CloseVerifiedAt: "2026-01-01T00:00:00Z"},
			{Type: "child", URL: "https://x/issues/1"},
			{Type: "related", URL: "https://x/issues/4"},
		},
	}
	got := CleanupAbandonUnresolvedChildren(record, map[string]bool{"c2": true})
	if want := []string{"c1", "https://x/issues/2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	record.IssueURL = ""
	got = CleanupAbandonUnresolvedChildren(record, map[string]bool{"c1": true, "c2": true})
	if want := []string{"https://x/issues/2", "https://x/issues/1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("without own url, self link counts: got %v want %v", got, want)
	}
	if got := CleanupAbandonUnresolvedChildren(model.IssueOpsRecord{}, nil); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
}

func TestValidateCleanupAbandonReason(t *testing.T) {
	if err := ValidateCleanupAbandonReason("  duplicate of #12  "); err != nil {
		t.Fatalf("plain reason rejected: %v", err)
	}
	if err := ValidateCleanupAbandonReason(strings.Repeat("a", CleanupAbandonReasonLimit)); err != nil {
		t.Fatalf("limit-sized reason rejected: %v", err)
	}
	for name, reason := range map[string]string{
		"empty":        "",
		"blank":        " \t ",
		"too long":     strings.Repeat("a", CleanupAbandonReasonLimit+1),
		"newline":      "a\nb",
		"del":          "a\x7fb",
		"nul":          "a\x00b",
		"double quote": `say "hi"`,
		"dollar":       "cost $5",
		"backtick":     "run `x`",
		"pipe":         "a|b",
		"semicolon":    "a;b",
		"tilde":        "~root",
		"glob":         "a*b",
	} {
		if err := ValidateCleanupAbandonReason(reason); err == nil {
			t.Errorf("%s: reason %q accepted", name, reason)
		}
	}
	err := ValidateCleanupAbandonReason("a;b")
	if err == nil || !strings.Contains(err.Error(), "';'") {
		t.Fatalf("error must name the offending rune: %v", err)
	}
	if err := ValidateCleanupAbandonReason("a\tb"); err == nil || !strings.Contains(err.Error(), "control") {
		t.Fatalf("tab inside is a control char: %v", err)
	}
}

func TestCleanupAbandonTargetsPrefersExecutionWorkspace(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-1", Repo: "acme/r", Branch: " rec-branch ", WorktreePath: " /rec ", Phase: model.IssueOpsPhaseDone}
	inv := CleanupAbandonTargets(record)
	if inv.LeaseStatus != "none" || inv.Branch != "rec-branch" || inv.WorktreeRoot != "/rec" || inv.Phase != string(model.IssueOpsPhaseDone) || inv.Repo != "acme/r" || inv.PendingOperationID != "" {
		t.Fatalf("record-only inventory: %+v", inv)
	}
	record.Execution = &model.Execution{
		Lease:     model.WriteLease{Status: model.LeaseStatusReleased},
		Workspace: model.Workspace{Root: " /exec ", Branch: " exec-branch "},
		Pending:   &model.ExternalIntent{OperationID: " op "},
	}
	inv = CleanupAbandonTargets(record)
	if inv.LeaseStatus != "released" || inv.Branch != "exec-branch" || inv.WorktreeRoot != "/exec" || inv.PendingOperationID != "op" {
		t.Fatalf("execution inventory: %+v", inv)
	}
	record.Execution.Workspace = model.Workspace{}
	inv = CleanupAbandonTargets(record)
	if inv.Branch != "rec-branch" || inv.WorktreeRoot != "/rec" {
		t.Fatalf("blank execution workspace must fall back to record: %+v", inv)
	}
}

func TestCleanupAbandonResolvedChildrenCountsOnlyDone(t *testing.T) {
	got := CleanupAbandonResolvedChildren([]model.IssueOpsRecord{
		{ID: "done", Phase: model.IssueOpsPhaseDone},
		{ID: "open", Phase: model.IssueOpsPhaseImplement},
	})
	if !got["done"] || got["open"] || len(got) != 1 {
		t.Fatalf("resolved: %v", got)
	}
}

func TestCleanupAbandonPendingErrorBlocksOnlyWhenIntentRemains(t *testing.T) {
	if got := CleanupAbandonPendingError(model.IssueOpsRecord{ID: "io-1"}); got != "" {
		t.Fatalf("no execution = %q", got)
	}
	if got := CleanupAbandonPendingError(model.IssueOpsRecord{ID: "io-1", Execution: &model.Execution{}}); got != "" {
		t.Fatalf("no pending intent = %q", got)
	}
	record := model.IssueOpsRecord{ID: "io-1", Execution: &model.Execution{Pending: &model.ExternalIntent{Kind: "remote_pr_create"}}}
	got := CleanupAbandonPendingError(record)
	if !strings.Contains(got, `"remote_pr_create"`) || !strings.Contains(got, "issueops execution reconcile --id io-1 --preview --json") {
		t.Fatalf("pending guidance = %q", got)
	}
}

func TestPlanCleanupAbandonRemoteReads(t *testing.T) {
	all := model.CleanupAbandonRequest{ClosePR: true, CloseIssue: true, DeleteRemoteBranch: true}
	if got := PlanCleanupAbandonRemoteReads(model.IssueOpsRecord{}, model.CleanupAbandonRequest{}, "task", true); got != (CleanupAbandonRemoteReads{}) {
		t.Fatalf("no request, no reads: %+v", got)
	}
	record := model.IssueOpsRecord{
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "u"},
		IssueURL:       "https://x/issues/1",
		BranchPrepare:  &model.IssueOpsBranchPrepare{BaseBranch: " main "},
	}
	got := PlanCleanupAbandonRemoteReads(record, all, "task", true)
	if got != (CleanupAbandonRemoteReads{Artifact: true, Issue: true, Branch: true}) {
		t.Fatalf("all satisfiable: %+v", got)
	}
	got = PlanCleanupAbandonRemoteReads(record, all, "task", false)
	if got.Artifact || got.Issue || got.ArtifactMissing != "remote_provider_unavailable" || got.IssueMissing != "remote_provider_unavailable" || !got.Branch {
		t.Fatalf("provider unavailable must not block the branch read: %+v", got)
	}
	got = PlanCleanupAbandonRemoteReads(model.IssueOpsRecord{IssueURL: "  "}, all, "", true)
	if got.ArtifactMissing != "remote_artifact_required" || got.IssueMissing != "issue_url_required" || got.BranchMissing != "branch_recorded" {
		t.Fatalf("missing prerequisites: %+v", got)
	}
	got = PlanCleanupAbandonRemoteReads(record, model.CleanupAbandonRequest{DeleteRemoteBranch: true}, "main", true)
	if got.Branch || got.BranchMissing != "branch_not_base" {
		t.Fatalf("base branch must not be deleted: %+v", got)
	}
}

func TestBuildCleanupAbandonRemotePreviewPlansEffectsAndInventory(t *testing.T) {
	req := model.CleanupAbandonRequest{ClosePR: true, CloseIssue: true, DeleteRemoteBranch: true}
	plan := CleanupAbandonRemoteReads{Artifact: true, Issue: true, Branch: true}
	inv, obs := BuildCleanupAbandonRemotePreview(req, model.CleanupAbandonInventory{}, plan, CleanupAbandonRemoteFacts{ArtifactState: "OPEN", IssueState: "open", BranchOID: "abc"})
	if !inv.ClosePR || !inv.CloseIssue || !inv.DeleteRemoteBranch || inv.RemoteBranchOID != "abc" {
		t.Fatalf("inventory: %+v", inv)
	}
	if want := []string{"close_pr", "close_issue", "remote_branch_delete"}; !reflect.DeepEqual(obs.RemoteEffects, want) || len(obs.RemoteMissing) != 0 {
		t.Fatalf("effects %v missing %v", obs.RemoteEffects, obs.RemoteMissing)
	}
	if obs.RemoteArtifactState != "OPEN" || obs.IssueState != "open" {
		t.Fatalf("states: %+v", obs)
	}

	_, obs = BuildCleanupAbandonRemotePreview(req, model.CleanupAbandonInventory{}, plan, CleanupAbandonRemoteFacts{ArtifactState: " Closed ", IssueState: " CLOSED ", BranchOID: ""})
	if want := []string{"close_pr:already_closed", "close_issue:already_closed", "remote_branch_delete:absent"}; !reflect.DeepEqual(obs.RemoteEffects, want) {
		t.Fatalf("idempotent effects: %v", obs.RemoteEffects)
	}
}

func TestBuildCleanupAbandonRemotePreviewRecordsEveryRefusalAndDropsEffects(t *testing.T) {
	req := model.CleanupAbandonRequest{ClosePR: true, CloseIssue: true, DeleteRemoteBranch: true}
	boom := errors.New("x")
	plan := CleanupAbandonRemoteReads{Artifact: true, Issue: true, Branch: true}
	_, obs := BuildCleanupAbandonRemotePreview(req, model.CleanupAbandonInventory{}, plan, CleanupAbandonRemoteFacts{ArtifactError: boom, IssueError: boom, BranchError: boom})
	want := []string{"remote_artifact_readable", "issue_readable", "remote_branch_readable"}
	if !reflect.DeepEqual(obs.RemoteMissing, want) || obs.RemoteEffects != nil {
		t.Fatalf("read errors: missing=%v effects=%v", obs.RemoteMissing, obs.RemoteEffects)
	}

	// A merged artifact refuses; other effects are still previewed but not reported as plan.
	_, obs = BuildCleanupAbandonRemotePreview(model.CleanupAbandonRequest{ClosePR: true, CloseIssue: true}, model.CleanupAbandonInventory{}, plan, CleanupAbandonRemoteFacts{ArtifactState: "MERGED", IssueState: "open"})
	if !reflect.DeepEqual(obs.RemoteMissing, []string{"remote_artifact_unmerged"}) || obs.RemoteEffects != nil || obs.RemoteArtifactState != "MERGED" {
		t.Fatalf("merged: %+v", obs)
	}

	// Plan-level refusals are reported verbatim and the branch OID is not copied.
	planned := CleanupAbandonRemoteReads{ArtifactMissing: "remote_artifact_required", IssueMissing: "issue_url_required", BranchMissing: "branch_not_base"}
	inv, obs := BuildCleanupAbandonRemotePreview(req, model.CleanupAbandonInventory{}, planned, CleanupAbandonRemoteFacts{BranchOID: "zzz"})
	if want := []string{"remote_artifact_required", "issue_url_required", "branch_not_base"}; !reflect.DeepEqual(obs.RemoteMissing, want) || inv.RemoteBranchOID != "" {
		t.Fatalf("plan refusals: %+v inv=%+v", obs, inv)
	}

	// No remote effects requested: nothing is planned or recorded.
	inv, obs = BuildCleanupAbandonRemotePreview(model.CleanupAbandonRequest{}, model.CleanupAbandonInventory{}, CleanupAbandonRemoteReads{}, CleanupAbandonRemoteFacts{BranchOID: "zzz"})
	if obs.RemoteEffects != nil || obs.RemoteMissing != nil || inv.ClosePR || inv.RemoteBranchOID != "" {
		t.Fatalf("empty request: %+v %+v", obs, inv)
	}
}

func TestArmCleanupAbandonReleasesClaimableLease(t *testing.T) {
	attempt := model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationAbandon, Token: covAbandonToken, StartedAt: "2026-01-02T03:04:05Z"}
	failure := model.IssueOpsCleanupAbandonFailure{Step: model.CleanupFailureStepApplying}
	record := model.IssueOpsRecord{Execution: &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusClaimable, ClaimTokenSHA256: "tok"}}}
	next, err := ArmCleanupAbandon(record, attempt, failure)
	if err != nil {
		t.Fatal(err)
	}
	if next.CleanupAttempt == nil || *next.CleanupAttempt != attempt || next.CleanupAbandonFailure == nil || next.CleanupAbandonFailure.Step != "applying" || next.UpdatedAt != attempt.StartedAt {
		t.Fatalf("armed record: %+v", next)
	}
	lease := next.Execution.Lease
	if lease.Status != model.LeaseStatusReleased || lease.ClaimTokenSHA256 != "" || lease.ReleasedAt != attempt.StartedAt {
		t.Fatalf("lease not released: %+v", lease)
	}
	if record.Execution.Lease.Status != model.LeaseStatusClaimable || record.Execution.Lease.ClaimTokenSHA256 != "tok" {
		t.Fatal("input record's execution must not be mutated")
	}

	record.Execution.Lease.Status = model.LeaseStatusReleased
	record.Execution.Lease.ReleasedAt = "earlier"
	next, err = ArmCleanupAbandon(record, attempt, failure)
	if err != nil || next.Execution.Lease.ReleasedAt != "earlier" {
		t.Fatalf("non-claimable lease untouched: %v %+v", err, next.Execution.Lease)
	}
	next, err = ArmCleanupAbandon(model.IssueOpsRecord{}, attempt, failure)
	if err != nil || next.Execution != nil || next.CleanupAttempt == nil {
		t.Fatalf("record without execution: %v %+v", err, next)
	}
}

func TestArmCleanupAbandonRejections(t *testing.T) {
	attempt := model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationAbandon, Token: covAbandonToken, StartedAt: "2026-01-02T03:04:05Z"}
	finish := attempt
	finish.Operation = model.CleanupOperationFinish
	record := model.IssueOpsRecord{}
	if got, err := ArmCleanupAbandon(record, finish, model.IssueOpsCleanupAbandonFailure{}); err == nil || got.CleanupAttempt != nil || got.CleanupAbandonFailure != nil {
		t.Fatalf("non-abandon operation: %v %+v", err, got)
	}
	owned := model.IssueOpsRecord{CleanupAttempt: &model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationFinish, Token: strings.Repeat("1", 64), StartedAt: "2026-01-01T00:00:00Z"}}
	if got, err := ArmCleanupAbandon(owned, attempt, model.IssueOpsCleanupAbandonFailure{}); err == nil || got.CleanupAbandonFailure != nil || got.CleanupAttempt.Operation != model.CleanupOperationFinish {
		t.Fatalf("finish owns cycle: %v", err)
	}
	bad := attempt
	bad.Token = "short"
	if _, err := ArmCleanupAbandon(record, bad, model.IssueOpsCleanupAbandonFailure{}); err == nil {
		t.Fatal("invalid token accepted")
	}
}

func TestApplyCleanupAbandonFailureDrainsAttemptOnlyWhenDrained(t *testing.T) {
	attempt := &model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationAbandon, Token: covAbandonToken, StartedAt: "2026-01-01T00:00:00Z"}
	failure := model.IssueOpsCleanupAbandonFailure{Step: "branch_delete", At: "2026-02-02T00:00:00Z"}
	record := model.IssueOpsRecord{CleanupAttempt: attempt, UpdatedAt: "old"}
	kept := ApplyCleanupAbandonFailure(record, failure, false)
	if kept.CleanupAttempt != attempt || kept.CleanupAbandonFailure == nil || kept.CleanupAbandonFailure.Step != "branch_delete" || kept.UpdatedAt != failure.At {
		t.Fatalf("undrained: %+v", kept)
	}
	drained := ApplyCleanupAbandonFailure(record, failure, true)
	if drained.CleanupAttempt != nil || drained.CleanupAbandonFailure.At != failure.At || drained.UpdatedAt != failure.At {
		t.Fatalf("drained: %+v", drained)
	}
	if record.CleanupAttempt == nil || record.CleanupAbandonFailure != nil {
		t.Fatal("input record mutated")
	}
}

func TestBuildCleanupAbandonPreviewReportsLeaseBranchAndReceiptGates(t *testing.T) {
	failure := covAbandonReceipt("branch_delete")
	failure.WorktreeHead, failure.BranchOID = "h", "h"
	record := model.IssueOpsRecord{ID: "io-1", Branch: "task", WorktreePath: "/w",
		Execution:             &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusActive}},
		CleanupAbandonFailure: failure,
		IssueURL:              "https://x/issues/1",
		ChildCycles:           []model.IssueOpsChildCycleRef{{CycleID: "child-1"}},
	}
	inv := model.CleanupAbandonInventory{Branch: "task", WorktreeRoot: "/w", BranchOID: "other"}
	_, res := BuildCleanupAbandonPreview(record, model.CleanupAbandonRequest{Reason: " why "}, inv, CleanupAbandonObservation{
		BranchObservable: true, RegistryObservable: true, BranchCheckedOutElsewhere: true, FailureEvidence: covAbandonEvidence,
	})
	want := "lease_terminal,no_children,branch_checked_out_elsewhere,cleanup_failure_inventory"
	if got := strings.Join(res.Missing, ","); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if res.OK || !res.Preview || res.Reason != "why" || !reflect.DeepEqual(res.UnresolvedChildren, []string{"child-1"}) || !res.BranchPresent {
		t.Fatalf("result: %+v", res)
	}

	_, res = BuildCleanupAbandonPreview(model.IssueOpsRecord{ID: "io-2", WorktreePath: "/w"}, model.CleanupAbandonRequest{Reason: "r", Apply: true},
		model.CleanupAbandonInventory{Branch: "task", BranchOID: "b"}, CleanupAbandonObservation{BranchObservable: true})
	if got := strings.Join(res.Missing, ","); got != "worktree_registry_observable" || res.Preview {
		t.Fatalf("unobservable registry: %v preview=%t", res.Missing, res.Preview)
	}
}

func TestBuildCleanupAbandonPreviewPartialBranchRetryRestoresRecordedHead(t *testing.T) {
	failure := covAbandonReceipt("branch_delete")
	failure.WorktreeHead, failure.BranchOID = "oid", "oid"
	record := model.IssueOpsRecord{ID: "io-1", Branch: "task", WorktreePath: "/w", CleanupAbandonFailure: failure}
	inv := model.CleanupAbandonInventory{Branch: "task", BranchOID: "oid"}
	gotInv, res := BuildCleanupAbandonPreview(record, model.CleanupAbandonRequest{Reason: "r"}, inv, CleanupAbandonObservation{
		BranchObservable: true, RegistryObservable: true, FailureEvidence: covAbandonEvidence,
	})
	if !res.OK || res.WorktreeHead != "oid" || gotInv.WorktreeHead != "oid" {
		t.Fatalf("recorded head must carry into result and inventory: %+v inv=%+v", res, gotInv)
	}
	// Without the retry receipt the head stays empty.
	record.CleanupAbandonFailure = nil
	gotInv, res = BuildCleanupAbandonPreview(record, model.CleanupAbandonRequest{Reason: "r"}, inv, CleanupAbandonObservation{BranchObservable: true, RegistryObservable: true})
	if res.WorktreeHead != "" || gotInv.WorktreeHead != "" {
		t.Fatalf("head leaked: %+v", res)
	}
}

func TestBuildCleanupAbandonPreviewForwardsRemoteDiagnostics(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-1", Branch: "task", WorktreePath: "/w"}
	inv := model.CleanupAbandonInventory{Branch: "task"}
	_, res := BuildCleanupAbandonPreview(record, model.CleanupAbandonRequest{Reason: "r", ClosePR: true}, inv, CleanupAbandonObservation{
		BranchObservable: true, RemoteEffects: []string{"close_pr"}, RemoteArtifactState: "open", IssueState: "closed",
		Occupants: []model.CleanupWorkspaceProcess{{PID: 7}},
	})
	if !res.OK || !reflect.DeepEqual(res.RemoteEffects, []string{"close_pr"}) || res.RemoteArtifactState != "open" || res.IssueState != "closed" || len(res.WorkspaceProcesses) != 1 {
		t.Fatalf("remote diagnostics: %+v", res)
	}
}
