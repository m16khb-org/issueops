package issueops

import (
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestValidateRecordInvariantsRequiresCompletedIssueURLMatch(t *testing.T) {
	completed := &issueopscontract.IssueOpsIssueCreateIntent{
		Status:       issueopscontract.IssueCreateIntentCompleted,
		CanonicalURL: "https://example.test/issues/1",
	}
	for _, test := range []struct {
		name      string
		url       string
		wantError bool
	}{
		{name: "matching", url: completed.CanonicalURL},
		{name: "whitespace normalized", url: "  " + completed.CanonicalURL + "  "},
		{name: "mismatched", url: "https://example.test/issues/2", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := issueopscontract.IssueOpsRecord{IssueURL: test.url, IssueCreateIntent: completed}
			err := ValidateRecordInvariants(record)
			if (err != nil) != test.wantError {
				t.Fatalf("ValidateRecordInvariants error = %v, wantError = %t", err, test.wantError)
			}
		})
	}
}

func TestValidateRecordInvariantsRejectsPlanPrepEvidenceAndWaiverConflict(t *testing.T) {
	cases := []issueopscontract.IssueOpsPlanPrepItem{
		{Status: "evidence", Evidence: []string{"source"}, WaiveReason: "not needed"},
		{Status: "waived"},
		{Status: "waived", Evidence: []string{"source"}, WaiveReason: "not needed"},
	}
	for index, item := range cases {
		record := issueopscontract.IssueOpsRecord{
			PlanPrep: &issueopscontract.IssueOpsPlanPrep{PriorDecisions: item},
		}
		if err := ValidateRecordInvariants(record); err == nil {
			t.Fatalf("case %d accepted invalid plan preparation: %+v", index, item)
		}
	}
}
