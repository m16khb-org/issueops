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

func TestCapturedOutputPreservesTruncationAfterRedaction(t *testing.T) {
	for _, value := range []string{"", "safe\n", "token=fake-value\n", strings.Repeat("가", 30000)} {
		got := capturedOutput(value, true)
		if !utf8.ValidString(got) || !strings.HasSuffix(got, "\n<truncated>\n") || len(got) > 32*1024+len("\n<truncated>\n") || strings.Contains(got, "fake-value") {
			t.Errorf("capture redaction/budget contract failed for %d bytes", len(value))
		}
	}
}

func TestOutputBudgetBoundaryIsInclusive(t *testing.T) {
	const limit = 32 * 1024
	for _, size := range []int{5, limit} {
		text := strings.Repeat("a", size)
		if got := budgetOutput(text); got != text {
			t.Errorf("budgetOutput altered %d bytes within the limit", size)
		}
		if got := capturedOutput(text, false); got != text {
			t.Errorf("capturedOutput altered %d untruncated bytes within the limit", size)
		}
	}
	if got := capturedOutput(strings.Repeat("a", limit), true); !strings.HasSuffix(got, "\n<truncated>\n") {
		t.Errorf("executor truncation marker lost for output exactly at the limit")
	}
	if got := budgetOutput(strings.Repeat("a", limit+1)); !strings.HasSuffix(got, "\n<truncated>\n") {
		t.Errorf("over-limit output was not marked truncated")
	}
}
