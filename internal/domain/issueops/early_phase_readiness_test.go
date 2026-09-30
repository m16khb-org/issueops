package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPlanReadinessMissingKeepsIntentBeforeIssueAndPreparation(t *testing.T) {
	if got := PlanReadinessMissing(model.IssueOpsRecord{}); !reflect.DeepEqual(got, []string{"intent_contract", "issue_url"}) {
		t.Fatalf("empty plan readiness=%v", got)
	}
	record := model.IssueOpsRecord{Intent: &model.IssueOpsIntentContract{RawRequest: "ask", InterpretedIntent: "outcome", SuccessCriteria: []string{"pass"}}}
	want := []string{"issue_url", "plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey"}
	if got := PlanReadinessMissing(record); !reflect.DeepEqual(got, want) {
		t.Fatalf("standard plan readiness=%v, want %v", got, want)
	}
}

func TestGrillReadinessMissingCombinesPlanAndReviewEvidence(t *testing.T) {
	record := model.IssueOpsRecord{Intent: &model.IssueOpsIntentContract{IntentClass: "trivial"}}
	want := []string{"issue_url", "branch", "split_decision", "domain_review"}
	if got := GrillReadinessMissing(record); !reflect.DeepEqual(got, want) {
		t.Fatalf("trivial grill readiness=%v, want %v", got, want)
	}
}
