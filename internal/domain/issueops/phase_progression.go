package issueops

import (
	"fmt"

	issueopscontract "issueops/internal/contract/issueops"
)

// ValidatePhaseProgression rejects a move out of the done phase and any move
// to an earlier phase. Staying in or moving past the current phase is
// allowed.
func ValidatePhaseProgression(from, to issueopscontract.Phase) error {
	if from == issueopscontract.PhaseDone {
		return fmt.Errorf("cannot leave done phase")
	}
	if PhaseRank(to) < PhaseRank(from) {
		return fmt.Errorf("cannot move issueops phase backward from %s to %s", from, to)
	}
	return nil
}
