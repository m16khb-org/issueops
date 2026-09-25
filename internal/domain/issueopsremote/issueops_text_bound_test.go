package remote

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundedIssueOpsTextUTF8Safe(t *testing.T) {
	got := boundedIssueOpsText(strings.Repeat("가", 200))
	if !utf8.ValidString(got) {
		t.Fatalf("bounded text is not valid UTF-8: %q", got)
	}
	if len(got) > 400+len("...[truncated]") {
		t.Fatalf("bounded text length = %d", len(got))
	}
}
