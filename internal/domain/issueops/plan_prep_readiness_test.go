package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPlanPrepGateAppliesOnlyToNonTrivialIntent(t *testing.T) {
	if PlanPrepGateApplies(model.IssueOpsRecord{}) {
		t.Fatal("missing intent should not hide the intent_contract gate")
	}
	if PlanPrepGateApplies(model.IssueOpsRecord{Intent: &model.IssueOpsIntentContract{IntentClass: " Trivial "}}) {
		t.Fatal("trivial intent should skip plan preparation")
	}
	if !PlanPrepGateApplies(model.IssueOpsRecord{Intent: &model.IssueOpsIntentContract{IntentClass: "architecture"}}) {
		t.Fatal("non-trivial intent requires plan preparation")
	}
}

func TestPlanPrepMissingPreservesOrderedKeysAndWaiverRules(t *testing.T) {
	want := []string{"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey"}
	if got := PlanPrepMissing(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing plan prep = %v", got)
	}
	prep := &model.IssueOpsPlanPrep{
		PriorDecisions: model.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"adr"}},
		RelatedIssues:  model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "  internal only  "},
		WebResearch:    model.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"  "}},
	}
	if got := PlanPrepMissing(prep); !reflect.DeepEqual(got, []string{"plan_prep_web_research", "plan_prep_codebase_survey"}) {
		t.Fatalf("invalid item keys = %v", got)
	}
	prep.WebResearch.Evidence = []string{"\x00"}
	if got := PlanPrepMissing(prep); !reflect.DeepEqual(got, []string{"plan_prep_web_research", "plan_prep_codebase_survey"}) {
		t.Fatalf("NUL evidence keys = %v", got)
	}
}
