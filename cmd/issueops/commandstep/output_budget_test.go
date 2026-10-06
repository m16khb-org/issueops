package commandstep

import (
	selfverifydomain "issueops/internal/domain/selfverify"
	"strings"
	"testing"
)

func TestTailWithBudgetMarksTruncation(t *testing.T) {
	out, truncated, original := selfverifydomain.TailWithBudget(strings.Repeat("x", 100), 40)
	if !truncated || original != 100 {
		t.Fatalf("expected truncation metadata, got truncated=%v original=%d", truncated, original)
	}
	if len(out) > 40 || !strings.Contains(out, "truncated") {
		t.Fatalf("unexpected bounded output: len=%d out=%q", len(out), out)
	}
}
