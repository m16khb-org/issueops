package policy

import (
	"strings"
	"testing"
)

func TestBoundedDiagnosticUTF8Safe(t *testing.T) {
	if got, want := BoundedDiagnostic(strings.Repeat("가", 10), 16), "가가가가가...[truncated]"; got != want {
		t.Fatalf("BoundedDiagnostic = %q, want %q", got, want)
	}
}
