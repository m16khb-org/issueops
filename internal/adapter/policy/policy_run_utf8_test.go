package policy

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBudgetOutputUTF8Safe(t *testing.T) {
	got := budgetOutput(strings.Repeat("가", 11000))
	if !utf8.ValidString(got) {
		t.Fatalf("budgeted output is not valid UTF-8")
	}
	if !strings.Contains(got, "<truncated>") {
		t.Fatalf("budgeted output lost the truncation marker")
	}
	if len(got) > 32*1024+len("\n<truncated>\n") {
		t.Fatalf("budgeted output length = %d", len(got))
	}
}
