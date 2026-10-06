package llmeval

import (
	selfverifydomain "issueops/internal/domain/selfverify"
	"strings"
)

const selfVerifyLLMEvalErrorBudgetBytes = 512

func BoundedLLMEvalError(prefix string, err error, output string) string {
	message := prefix + ": " + err.Error()
	output = strings.TrimSpace(output)
	if output != "" {
		message += ": " + output
	}
	bounded, _, _ := selfverifydomain.TailWithBudget(message, selfVerifyLLMEvalErrorBudgetBytes)
	return bounded
}
