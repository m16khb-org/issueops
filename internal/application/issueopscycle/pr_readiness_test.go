package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func TestLocalPRReadinessMissingPreservesOrderedRecordGates(t *testing.T) {
	observations := cycleport.ReadinessObservations{
		PlanInLinkedWorktree: func(model.IssueOpsRecord) bool { return true },
	}
	want := []string{"issue_url", "branch", "branch_prepare", "intent_contract", "design_review", "plan_path", "worktree_path", "ai_slop_clean", "project_docs_review"}
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
