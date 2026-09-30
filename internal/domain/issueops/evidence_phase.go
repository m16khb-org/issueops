package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

func ValidateEvidenceRecordingPhase(phase model.IssueOpsPhase, kind string) error {
	if IssueOpsPhaseRank(phase) < IssueOpsPhaseRank(model.IssueOpsPhaseImplement) {
		return fmt.Errorf("%s can only be recorded from the implement phase onward (current: %s)", kind, phase)
	}
	return nil
}
