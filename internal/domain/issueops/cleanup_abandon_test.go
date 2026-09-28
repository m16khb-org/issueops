package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// A remote failure precedes every local deletion. Accepting removed/replaced
// resources here would turn the retry receipt into authority over a new target.
func TestCleanupAbandonRemoteRetryRequiresUnchangedLocalInventory(t *testing.T) {
	for _, step := range []string{"close_pr", "close_issue", "remote_branch_delete"} {
		for _, shape := range []struct {
			name             string
			worktree, branch bool
		}{
			{"absent", false, false}, {"worktree", true, false}, {"branch", false, true}, {"paired", true, true},
		} {
			t.Run(step+"/"+shape.name, func(t *testing.T) {
				failure := &model.IssueOpsCleanupAbandonFailure{Step: step, Branch: "task", Fingerprint: strings.Repeat("a", 64), RecordSHA: strings.Repeat("b", 64), InventorySHA256: strings.Repeat("c", 64)}
				inventory := model.CleanupAbandonInventory{Branch: "task", WorktreePresent: shape.worktree}
				if shape.worktree {
					failure.WorktreeHead = "old"
					inventory.WorktreeHead = "old"
				}
				if shape.branch {
					failure.BranchOID = "old"
					inventory.BranchOID = "old"
				}
				record := model.IssueOpsRecord{CleanupAbandonFailure: failure}
				evidence := CleanupAbandonFailureEvidence{SameWorktree: true, SealMatches: true}
				if !CleanupAbandonFailureInventoryMatches(record, inventory, evidence) {
					t.Fatal("unchanged inventory must be retryable")
				}
				for _, change := range []struct {
					name   string
					mutate func(*model.CleanupAbandonInventory)
				}{
					{"worktree presence", func(i *model.CleanupAbandonInventory) {
						i.WorktreePresent = !i.WorktreePresent
						if i.WorktreePresent {
							i.WorktreeHead = "old"
						}
					}},
					{"branch presence", func(i *model.CleanupAbandonInventory) {
						if i.BranchOID == "" {
							i.BranchOID = "old"
						} else {
							i.BranchOID = ""
						}
					}},
					{"replaced worktree", func(i *model.CleanupAbandonInventory) { i.WorktreePresent = true; i.WorktreeHead = "new" }},
					{"replaced branch", func(i *model.CleanupAbandonInventory) { i.BranchOID = "new" }},
				} {
					t.Run(change.name, func(t *testing.T) {
						next := inventory
						change.mutate(&next)
						if CleanupAbandonFailureInventoryMatches(record, next, evidence) {
							t.Fatal("changed local inventory accepted")
						}
					})
				}
				for _, bad := range []CleanupAbandonFailureEvidence{{SameWorktree: false, SealMatches: true}, {SameWorktree: true, SealMatches: false}} {
					if CleanupAbandonFailureInventoryMatches(record, inventory, bad) {
						t.Fatal("invalid receipt evidence accepted")
					}
				}
			})
		}
	}
}

func TestCleanupAbandonPreviewKeepsUnknownResourcesBlocked(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-example", Branch: "task", WorktreePath: "/task"}
	inventory := CleanupAbandonTargets(record)
	observed := CleanupAbandonObservation{WorktreeUnobservable: true}
	_, result := BuildCleanupAbandonPreview(record, model.CleanupAbandonRequest{Reason: "cancel task"}, inventory, observed)
	if result.OK || strings.Join(result.Missing, ",") != "worktree_observable,local_branch_observable" {
		t.Fatalf("unknown is not absence: %+v", result)
	}
	observed.WorktreeUnobservable = false
	observed.BranchObservable = true
	_, result = BuildCleanupAbandonPreview(record, model.CleanupAbandonRequest{Reason: "cancel task"}, inventory, observed)
	if !result.OK || len(result.Missing) != 0 {
		t.Fatalf("authoritative absence should pass: %+v", result)
	}
}

func TestCleanupAbandonPreviewPreservesAllRefusalReasons(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-example", Branch: "task", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/1"}}
	inventory := model.CleanupAbandonInventory{Branch: "task", WorktreePresent: true, WorktreeHead: "first", BranchOID: "second"}
	facts := CleanupAbandonObservation{
		WorktreeIdentityConflict: true, WorkspaceMissing: []string{"source_checkout"},
		PendingIntentError: "unresolved external intent", OrcaResidueError: "task still live", RemoteMissing: []string{"issue_readable"},
	}
	_, result := BuildCleanupAbandonPreview(record, model.CleanupAbandonRequest{}, inventory, facts)
	want := "reason_required,remote_artifact_unmerged,worktree_identity_conflict,worktree_canonical,worktree_branch_match,worktree_head,worktree_clean,source_checkout,local_branch_observable,worktree_registry_observable,local_residue_execution,local_branch_head,pending_intent_safe,orca_resources_absent,issue_readable"
	if got := strings.Join(result.Missing, ","); got != want {
		t.Fatalf("all ordered reasons: got=%s want=%s", got, want)
	}
	if result.OK || result.ReasonError == "" || result.PendingIntentError != facts.PendingIntentError || result.OrcaResidueError != facts.OrcaResidueError {
		t.Fatalf("diagnostics lost: %+v", result)
	}
}
