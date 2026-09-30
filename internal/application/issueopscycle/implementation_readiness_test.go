package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestImplementationReadinessMissingObservesPathsBeforeReview(t *testing.T) {
	var calls []string
	observations := cycleport.ReadinessObservations{
		WorktreePathValid:    func(string) bool { calls = append(calls, "worktree"); return false },
		PlanPathExists:       func(string, string) bool { calls = append(calls, "plan"); return false },
		PlanInLinkedWorktree: func(model.IssueOpsRecord) bool { calls = append(calls, "inside"); return false },
		WorkspaceMatches:     func(string, string) bool { calls = append(calls, "workspace"); return false },
		LinkedPlanDigest:     func(model.IssueOpsRecord) (string, error) { calls = append(calls, "digest"); return "", nil },
	}
	record := model.IssueOpsRecord{Repo: "source", WorktreePath: "tree", PlanPath: "plan.md"}
	missing := ImplementationReadinessMissing(record, true, observations)
	wantCalls := []string{"worktree", "plan", "inside"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("observation order=%v, want %v", calls, wantCalls)
	}
	want := []string{"issue_url", "branch", "branch_prepare", "intent_contract", "design_review", "worktree_exists", "plan_exists", "plan_in_worktree", "compatibility_review", "devils_advocate_review", "execution"}
	if !reflect.DeepEqual(missing, want) {
		t.Fatalf("missing=%v, want %v", missing, want)
	}
}
