package llmeval

import (
	selfverifydomain "issueops/internal/domain/selfverify"
	"strings"
)

const selfVerifyLLMEvalErrorBudgetBytes = 512

// BoundedLLMEvalError formats "prefix: err[: output]" and keeps only the tail of the
// message within the LLM evaluation error byte budget.
func BoundedLLMEvalError(prefix string, err error, output string) string {
	message := prefix + ": " + err.Error()
	output = strings.TrimSpace(output)
	if output != "" {
		message += ": " + output
	}
	bounded, _, _ := selfverifydomain.TailWithBudget(message, selfVerifyLLMEvalErrorBudgetBytes)
	return bounded
}
