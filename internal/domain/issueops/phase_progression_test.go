package issueops

import (
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestValidatePhaseProgression(t *testing.T) {
	for _, test := range []struct {
		from      issueopscontract.Phase
		to        issueopscontract.Phase
		wantError string
	}{
		{issueopscontract.PhasePlan, issueopscontract.PhaseImplement, ""},
		{issueopscontract.PhaseDone, issueopscontract.PhaseDone, "cannot leave done phase"},
		{issueopscontract.PhasePlan, issueopscontract.PhaseGrill, "cannot move issueops phase backward from plan to grill"},
	} {
		if got := errorText(ValidatePhaseProgression(test.from, test.to)); got != test.wantError {
			t.Fatalf("ValidatePhaseProgression(%s, %s) = %q, want %q", test.from, test.to, got, test.wantError)
		}
	}
}
