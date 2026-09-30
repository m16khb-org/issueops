package judgement

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundedOutputTextUTF8Safe(t *testing.T) {
	got := boundedOutputText(strings.Repeat("가", 500))
	if !utf8.ValidString(got) {
		t.Fatalf("bounded output is not valid UTF-8: %q", got)
	}
	if len(got) > 1000+len("...[truncated]") {
		t.Fatalf("bounded output length = %d", len(got))
	}
}
