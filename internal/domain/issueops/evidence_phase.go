package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

// ValidateEvidenceRecordingPhase fails when the evidence kind is recorded
// before the implement phase; evidence only counts from implement onward.
func ValidateEvidenceRecordingPhase(phase model.Phase, kind string) error {
	if PhaseRank(phase) < PhaseRank(model.PhaseImplement) {
		return fmt.Errorf("%s can only be recorded from the implement phase onward (current: %s)", kind, phase)
	}
	return nil
}
