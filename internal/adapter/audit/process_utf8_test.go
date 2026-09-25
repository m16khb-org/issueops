package audit

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundedProcessFieldUTF8Safe(t *testing.T) {
	got := boundedProcessField(strings.Repeat("가", 500))
	if !utf8.ValidString(got) {
		t.Fatalf("bounded field is not valid UTF-8: %q", got)
	}
	if len(got) > processDiagnosticLimit+len("...") {
		t.Fatalf("bounded field length = %d", len(got))
	}
}
