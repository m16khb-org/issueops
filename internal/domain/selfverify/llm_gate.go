package selfverify

import (
	"fmt"
	"strings"
)

func LLMEvalGateReasons(mode string, ok bool, score, targetScore float64, blockers []string) []string {
	if mode != "gate" {
		return nil
	}
	reasons := []string{}
	if !ok {
		reasons = append(reasons, "llm_eval_not_ok")
	}
	if score < targetScore {
		reasons = append(reasons, fmt.Sprintf("score %.2f below target %.2f", score, targetScore))
	}
	if len(blockers) > 0 {
		reasons = append(reasons, "blockers: "+strings.Join(blockers, "; "))
	}
	return reasons
}
