package issueops

import (
	"fmt"

	issueopscontract "issueops/internal/contract/issueops"
)

func ValidatePhaseProgression(from, to issueopscontract.Phase) error {
	if from == issueopscontract.PhaseDone {
		return fmt.Errorf("cannot leave done phase")
	}
	if PhaseRank(to) < PhaseRank(from) {
		return fmt.Errorf("cannot move issueops phase backward from %s to %s", from, to)
	}
	return nil
}
