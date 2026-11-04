package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

func ValidateEvidenceRecordingPhase(phase model.Phase, kind string) error {
	if PhaseRank(phase) < PhaseRank(model.PhaseImplement) {
		return fmt.Errorf("%s can only be recorded from the implement phase onward (current: %s)", kind, phase)
	}
	return nil
}
