package issueops

import (
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

func ExecutionReseedNextCommand(id string, generation uint64, mode, claimTokenPath string) string {
	switch issueops.ExecutionMode(mode) {
	case issueops.ExecutionModeOrca:
		return ExecutionResumeRecoveryCommand(id, generation)
	case issueops.ExecutionModeDirect:
		return domain.ReplacementClaimCommand(id, generation, claimTokenPath)
	default:
		return ""
	}
}
