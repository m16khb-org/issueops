package selfaugment

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/selfaugment"
	selfverifydomain "issueops/internal/domain/selfverify"
)

func ApplySelfVerifyLLMGate(result contract.SelfAugmentResult, targetScore float64) (contract.SelfAugmentResult, error) {
	if result.LLMEval == nil || result.LLMEval.Mode != "gate" {
		return result, nil
	}
	reasons := selfverifydomain.LLMEvalGateReasons(result.LLMEval.Mode, result.LLMEval.OK, result.LLMEval.Score, targetScore, result.LLMEval.Blockers)
	if len(reasons) == 0 {
		return result, nil
	}
	result.OK = false
	result.TerminationEligible = false
	result.Summary.TerminationEligible = false
	return result, fmt.Errorf("LLM evaluation gate failed: %s", strings.Join(reasons, "; "))
}
