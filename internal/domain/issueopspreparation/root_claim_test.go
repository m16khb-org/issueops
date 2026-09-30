package issueopspreparation

import "testing"

func TestCanonicalRootClaimConflictsOnlyWithOtherLifecycle(t *testing.T) {
	if CanonicalRootClaimConflict("self", "/worktree", "self", "/worktree", "") {
		t.Fatal("self claim conflicted")
	}
	if !CanonicalRootClaimConflict("self", "/worktree", "other", "", "/worktree") {
		t.Fatal("execution workspace claim was missed")
	}
	if CanonicalRootClaimConflict("self", "/worktree", "other", "/another", "") {
		t.Fatal("unrelated root conflicted")
	}
}
