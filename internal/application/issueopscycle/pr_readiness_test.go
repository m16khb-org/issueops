package issueopscycle

import (
	"reflect"
	"slices"
	"testing"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestLocalPRReadinessMissingPreservesOrderedRecordGates(t *testing.T) {
	observations := cycleport.ReadinessObservations{
		PlanInLinkedWorktree: func(model.IssueOpsRecord) bool { return true },
	}
	want := []string{"issue_url", "branch", "branch_prepare", "intent_contract", "architecture_review", "plan_path", "worktree_path", "ai_slop_clean", "project_docs_review"}
	if got := LocalPRReadinessMissing(model.IssueOpsRecord{}, observations); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
}

func TestReviewGateMissingMapsRecordEvidence(t *testing.T) {
	record := model.IssueOpsRecord{}
	if got := ImplementationReviewMissing(record, ""); got != "" {
		t.Fatalf("unprepared implementation review=%q", got)
	}
	if got := ProjectDocsReviewMissing(record, ""); got != "project_docs_review" {
		t.Fatalf("missing docs review=%q", got)
	}
	record.Execution = &model.Execution{}
	if got := ImplementationReviewMissing(record, ""); got != "implementation_review" {
		t.Fatalf("prepared implementation review=%q", got)
	}
}

func TestLocalPRReadinessMissingReportsPlanAndFeedbackDefects(t *testing.T) {
	record := model.IssueOpsRecord{
		WorktreePath: "/wt", PlanPath: ".issueops/plans/p.md", AISlopCleanAt: "t",
		Feedback: []model.IssueOpsFeedbackItem{{Classification: "contract_change"}},
	}
	var rootSeen, pathSeen string
	observations := cycleport.ReadinessObservations{
		PlanPathExists:       func(root, path string) bool { rootSeen, pathSeen = root, path; return false },
		PlanInLinkedWorktree: func(model.IssueOpsRecord) bool { return false },
	}
	got := LocalPRReadinessMissing(record, observations)
	for _, want := range []string{"plan_exists", "plan_in_worktree", "contract_feedback_issue_update"} {
		if !slices.Contains(got, want) {
			t.Fatalf("missing %q from %v", want, got)
		}
	}
	for _, absent := range []string{"worktree_path", "plan_path", "ai_slop_clean"} {
		if slices.Contains(got, absent) {
			t.Fatalf("%q reported although the record has it: %v", absent, got)
		}
	}
	if rootSeen != "/wt" || pathSeen != ".issueops/plans/p.md" {
		t.Fatalf("plan existence must be observed under the worktree: root=%q path=%q", rootSeen, pathSeen)
	}
}
