package selfverify

import (
	"fmt"
	"strings"
	"testing"

	"issueops/internal/domain/policy"
)

func TestCommandOutputMatchesOriginalFormatterAcrossChunks(t *testing.T) {
	for _, input := range []string{
		"", "abc", strings.Repeat("0123456789", 100),
		strings.Repeat("a가🙂", 200), strings.Repeat("\x80", 100) + "tail",
	} {
		data := []byte(input)
		for _, budget := range []int{-5, 0, 1, 2, 31, 49, 50, 51, 63, 64, 65, 99, 100, 101, len(input), len(input) + 1} {
			t.Run(fmt.Sprintf("bytes=%d/budget=%d", len(input), budget), func(t *testing.T) {
				for _, chunk := range []int{1, 2, 3, 7, 63, 64, 65, len(input) + 1} {
					output := NewCommandOutput(budget)
					for offset := 0; offset < len(data); offset += chunk {
						p := data[offset:min(offset+chunk, len(data))]
						n, err := output.Write(p)
						if err != nil || n != len(p) {
							t.Fatalf("Write=%d,%v want=%d", n, err, len(p))
						}
						prefix := input[:min(offset+chunk, len(data))]
						want, truncated, total := originalTailWithBudget(prefix, budget)
						if budget <= 0 {
							want, truncated, total = prefix, false, len(prefix)
						}
						got, gotTruncated, gotTotal := output.Result()
						if got != want || gotTruncated != truncated || gotTotal != total {
							t.Fatalf("chunk=%d offset=%d got=%q,%v,%d want=%q,%v,%d",
								chunk, offset, got, gotTruncated, gotTotal, want, truncated, total)
						}
					}
					want, truncated, total := originalTailWithBudget(input, budget)
					got, gotTruncated, gotTotal := TailWithBudget(input, budget)
					if got != want || gotTruncated != truncated || gotTotal != total {
						t.Fatalf("TailWithBudget=%q,%v,%d want=%q,%v,%d", got, gotTruncated, gotTotal, want, truncated, total)
					}
					want, truncated, total = input, false, len(input)
					if budget > 0 {
						want, truncated, total = TailWithBudget(input, budget)
					}
					got, gotTruncated, gotTotal = output.Result()
					if got != want || gotTruncated != truncated || gotTotal != total {
						t.Fatalf("Result=%q,%v,%d want=%q,%v,%d", got, gotTruncated, gotTotal, want, truncated, total)
					}
					if n, err := output.Write(nil); n != 0 || err != nil {
						t.Fatalf("empty Write=%d,%v", n, err)
					}
				}
			})
		}
	}
}

// The pre-capture formatter is an independent oracle for byte-exact compatibility.
func originalTailWithBudget(s string, max int) (string, bool, int) {
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
		next := max - len(marker)
		if next < 0 {
			return marker[:max], true, originalBytes
		}
		if next == tailBudget {
			return marker + tail, true, originalBytes
		}
		tailBudget = next
	}
}
