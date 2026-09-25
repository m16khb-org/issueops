package commandstep

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTailWithBudgetUTF8Safe(t *testing.T) {
	s := strings.Repeat("가", 500)
	got, truncated, originalBytes := TailWithBudget(s, 200)
	if !truncated || originalBytes != len(s) {
		t.Fatalf("TailWithBudget truncated=%v originalBytes=%d", truncated, originalBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("tail is not valid UTF-8: %q", got)
	}
	if len(got) > 200 {
		t.Fatalf("tail length = %d", len(got))
	}
	marker, tail, ok := strings.Cut(got, "\n")
	if !ok {
		t.Fatalf("tail has no marker line: %q", got)
	}
	want := fmt.Sprintf("[truncated: original_bytes=%d omitted_bytes=%d]", len(s), len(s)-len(tail))
	if marker != want {
		t.Fatalf("marker = %q, want %q", marker, want)
	}
}
