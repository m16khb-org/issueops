package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestWorkspaceSelectionPrefersWorktreeAndPreservesSourceOrder(t *testing.T) {
	matches := []CyclePathMatch{
		{Record: model.IssueOpsRecord{ID: "first", Phase: model.IssueOpsPhasePlan}, Repository: true},
		{Record: model.IssueOpsRecord{ID: "second", Phase: model.IssueOpsPhasePlan}, Repository: true},
		{Record: model.IssueOpsRecord{ID: "done", Phase: model.IssueOpsPhaseDone}, Worktree: true},
		{Record: model.IssueOpsRecord{ID: "specific", Phase: model.IssueOpsPhaseImplement}, Worktree: true},
	}
	if got, ok := SelectWorkspaceCycle(matches); !ok || got.ID != "specific" {
		t.Fatalf("worktree did not win: %+v %v", got, ok)
	}
	if got, ok := SelectWorkspaceCycle(matches[:3]); !ok || got.ID != "first" {
		t.Fatalf("source order changed: %+v %v", got, ok)
	}
	if _, ok := SelectWorkspaceCycle(matches[2:3]); ok {
		t.Fatal("done cycle selected")
	}
}

func TestPreparedCycleBaseRequiresNonblankPreparation(t *testing.T) {
	for _, prepared := range []*model.IssueOpsBranchPrepare{nil, {}, {BaseBranch: " \t"}} {
		if base, ok := PreparedCycleBase(model.IssueOpsRecord{BranchPrepare: prepared}); ok || base != "" {
			t.Fatalf("invalid prepared base=%q %v", base, ok)
		}
	}
	if base, ok := PreparedCycleBase(model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: " 78-parent "}}); !ok || base != "78-parent" {
		t.Fatalf("prepared base=%q %v", base, ok)
	}
}
