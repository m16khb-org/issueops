package selfverify

import (
	"fmt"

	"issueops/internal/domain/policy"
)

func TailWithBudget(s string, max int) (string, bool, int) {
	originalBytes := len(s)
	if max <= 0 {
		return "", originalBytes > 0, originalBytes
	}
	if originalBytes <= max {
		return s, false, originalBytes
	}
	tailBudget := max
	for {
		tail := policy.TailBytes(s, tailBudget)
		marker := fmt.Sprintf("[truncated: original_bytes=%d omitted_bytes=%d]\n", originalBytes, originalBytes-len(tail))
		tailBudgetNext := max - len(marker)
		if tailBudgetNext < 0 {
			return marker[:max], true, originalBytes
		}
		if tailBudgetNext == tailBudget {
			return marker + tail, true, originalBytes
		}
		tailBudget = tailBudgetNext
	}
}

func BudgetCommandOutput(s string, budget int) (string, bool, int) {
	if budget <= 0 {
		return s, false, len(s)
	}
	return TailWithBudget(s, budget)
}
