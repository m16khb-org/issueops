package selfverify

import (
	"fmt"
	"unicode/utf8"
)

// TailWithBudget keeps the tail of s within limit bytes, prefixing a truncation marker when bytes were
// omitted. It returns the text, whether it was truncated, and the original byte length.
// A non-positive limit drops all output.
func TailWithBudget(s string, limit int) (string, bool, int) {
	return tailWithOriginalBytes(s, len(s), limit)
}

// s contains at least the last limit raw bytes when originalBytes exceeds limit.
func tailWithOriginalBytes(s string, originalBytes, limit int) (string, bool, int) {
	if limit <= 0 {
		return "", originalBytes > 0, originalBytes
	}
	if originalBytes <= limit {
		return s, false, originalBytes
	}
	tailBudget := limit
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
		tailBudgetNext := limit - len(marker)
		if tailBudgetNext < 0 {
			return marker[:limit], true, originalBytes
		}
		if tailBudgetNext == tailBudget {
			return marker + tail, true, originalBytes
		}
		tailBudget = tailBudgetNext
	}
}
