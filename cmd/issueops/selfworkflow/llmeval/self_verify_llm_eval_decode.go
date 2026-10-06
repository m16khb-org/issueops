package llmeval

import (
	"strings"

	"issueops/cmd/issueops/commandstep"
)

const selfVerifyLLMEvalErrorBudgetBytes = 512

func BoundedLLMEvalError(prefix string, err error, output string) string {
	message := prefix + ": " + err.Error()
	output = strings.TrimSpace(output)
	if output != "" {
		message += ": " + output
	}
	bounded, _, _ := commandstep.TailWithBudget(message, selfVerifyLLMEvalErrorBudgetBytes)
	return bounded
}
