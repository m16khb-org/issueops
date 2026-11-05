package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestWorkspaceSelectionPrefersWorktreeAndPreservesSourceOrder(t *testing.T) {
	matches := []CyclePathMatch{
		{Record: model.Record{ID: "first", Phase: model.PhasePlan}, Repository: true},
		{Record: model.Record{ID: "second", Phase: model.PhasePlan}, Repository: true},
		{Record: model.Record{ID: "done", Phase: model.PhaseDone}, Worktree: true},
		{Record: model.Record{ID: "specific", Phase: model.PhaseImplement}, Worktree: true},
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
	for _, prepared := range []*model.BranchPrepare{nil, {}, {BaseBranch: " \t"}} {
		if base, ok := PreparedCycleBase(model.Record{BranchPrepare: prepared}); ok || base != "" {
			t.Fatalf("invalid prepared base=%q %v", base, ok)
		}
	}
	if base, ok := PreparedCycleBase(model.Record{BranchPrepare: &model.BranchPrepare{BaseBranch: " 78-parent "}}); !ok || base != "78-parent" {
		t.Fatalf("prepared base=%q %v", base, ok)
	}
}
