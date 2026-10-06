package selfverify

import (
	"fmt"
	"unicode/utf8"
)

func TailWithBudget(s string, max int) (string, bool, int) {
	return tailWithBudget(s, len(s), max)
}

// s contains at least the last max raw bytes when originalBytes exceeds max.
func tailWithBudget(s string, originalBytes, max int) (string, bool, int) {
	if max <= 0 {
		return "", originalBytes > 0, originalBytes
	}
	if originalBytes <= max {
		return s, false, originalBytes
	}
	tailBudget := max
	for {
		tail := ""
		if tailBudget > 0 {
			start := len(s) - min(len(s), tailBudget)
			if originalBytes > tailBudget {
				for i := 0; i < utf8.UTFMax-1 && start < len(s) && !utf8.RuneStart(s[start]); i++ {
					start++
				}
			}
			tail = s[start:]
		}
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
