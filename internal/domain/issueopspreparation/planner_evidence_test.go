package issueopspreparation

import (
	"reflect"
	"testing"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestPlannerGateDecisionUsesTypedEvidence(t *testing.T) {
	evidence := preparationcontract.PlannerEvidence{
		Intent:         preparationcontract.PlannerIntentEvidence{RawRequest: "r", InterpretedIntent: "i", SuccessCriteria: []string{"c"}},
		DesignReview:   preparationcontract.PlannerDesignEvidence{ProblemSummary: "p", ProposedDesign: "d", Verification: []string{"v"}, Approved: true},
		DevilsAdvocate: preparationcontract.PlannerDevilsAdvocateEvidence{Verdict: "pass", ReviewedPlanDigest: "abc", RecordedAt: "now"},
	}
	if got := MissingPlannerGateKeys(evidence); len(got) != 0 {
		t.Fatalf("complete evidence rejected: %v", got)
	}
	evidence.DesignReview.Approved = false
	if got := MissingPlannerGateKeys(evidence); !reflect.DeepEqual(got, []string{"design_review"}) {
		t.Fatalf("unapproved design = %v", got)
	}
}
