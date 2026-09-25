package issueops

import (
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestValidatePhaseProgression(t *testing.T) {
	for _, test := range []struct {
		from      issueopscontract.IssueOpsPhase
		to        issueopscontract.IssueOpsPhase
		wantError string
	}{
		{issueopscontract.IssueOpsPhasePlan, issueopscontract.IssueOpsPhaseImplement, ""},
		{issueopscontract.IssueOpsPhaseDone, issueopscontract.IssueOpsPhaseDone, "cannot leave done phase"},
		{issueopscontract.IssueOpsPhasePlan, issueopscontract.IssueOpsPhaseGrill, "cannot move issueops phase backward from plan to grill"},
	} {
		if got := errorText(ValidatePhaseProgression(test.from, test.to)); got != test.wantError {
			t.Fatalf("ValidatePhaseProgression(%s, %s) = %q, want %q", test.from, test.to, got, test.wantError)
		}
	}
}
