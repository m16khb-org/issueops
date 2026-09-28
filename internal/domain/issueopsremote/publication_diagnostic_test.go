package remote

import (
	"errors"
	"strings"
	"testing"
)

func TestPublicationDiagnosticPreservesFallbackRedactionAndByteLimit(t *testing.T) {
	if got := PublicationFailureDiagnostic(nil); got != "external operation failed" {
		t.Fatalf("fallback=%q", got)
	}
	if got := PublicationFailureDiagnostic(errors.New("  failure  ")); got != "failure" {
		t.Fatalf("trim=%q", got)
	}
	if got := PublicationFailureDiagnostic(errors.New("token=super-secret")); got != "<redacted>" {
		t.Fatalf("secret diagnostic=%q", got)
	}
	got := PublicationFailureDiagnostic(errors.New("https://internal.example/path " + strings.Repeat("x", 5000)))
	if strings.Contains(got, "super-secret") || strings.Contains(got, "internal.example") || len(got) != 4096 {
		t.Fatalf("redaction or bound failed: length=%d", len(got))
	}
}
