package llmeval

import (
	"fmt"
	"strings"

	"issueops/cmd/issueops/selfworkflow/model"
	selfverifydomain "issueops/internal/domain/selfverify"
)

func ApplySelfVerifyLLMGate(result model.SelfAugmentResult, targetScore float64) (model.SelfAugmentResult, error) {
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
