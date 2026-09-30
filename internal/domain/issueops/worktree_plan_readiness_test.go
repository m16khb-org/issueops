package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestWorktreePlanReadinessMissingPreservesOrderedGates(t *testing.T) {
	if got := WorktreePlanReadinessMissing(model.IssueOpsRecord{}, false, false, true); !reflect.DeepEqual(got, []string{"worktree_path"}) {
		t.Fatalf("empty paths=%v", got)
	}
	record := model.IssueOpsRecord{WorktreePath: "tree", PlanPath: "plan.md"}
	if got := WorktreePlanReadinessMissing(record, false, false, false); !reflect.DeepEqual(got, []string{"worktree_exists", "plan_exists", "plan_in_worktree"}) {
		t.Fatalf("broken paths=%v", got)
	}
	if got := WorktreePlanReadinessMissing(record, true, true, true); len(got) != 0 {
		t.Fatalf("valid paths=%v", got)
	}
}

func TestPlanExistenceRootPrefersLinkedWorktree(t *testing.T) {
	record := model.IssueOpsRecord{Repo: " repo ", WorktreePath: " tree "}
	if got := PlanExistenceRoot(record); got != "tree" {
		t.Fatalf("linked root=%q", got)
	}
	record.WorktreePath = " "
	if got := PlanExistenceRoot(record); got != "repo" {
		t.Fatalf("fallback root=%q", got)
	}
}
