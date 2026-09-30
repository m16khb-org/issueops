package llmeval

import (
	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

func ApplySelfVerifyLLMGate(result contract.SelfAugmentResult, target float64) (contract.SelfAugmentResult, error) {
	return domain.ApplySelfVerifyLLMGate(result, target)
}
