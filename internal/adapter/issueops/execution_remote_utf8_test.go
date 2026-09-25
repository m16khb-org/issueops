package issueops

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundedExecutionRemoteDiagnosticUTF8Safe(t *testing.T) {
	got := boundedExecutionRemoteDiagnostic(errors.New(strings.Repeat("가", 1400)))
	if !utf8.ValidString(got) {
		t.Fatalf("diagnostic is not valid UTF-8")
	}
	if len(got) > 4096 {
		t.Fatalf("diagnostic length = %d", len(got))
	}
}
