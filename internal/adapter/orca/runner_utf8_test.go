package orca

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestOrcaBoundedDiagnosticUTF8Safe(t *testing.T) {
	got := boundedDiagnostic(strings.Repeat("가", 500))
	if !utf8.ValidString(got) {
		t.Fatalf("bounded diagnostic is not valid UTF-8: %q", got)
	}
	if len(got) > 1024+len("...") {
		t.Fatalf("bounded diagnostic length = %d", len(got))
	}
}
