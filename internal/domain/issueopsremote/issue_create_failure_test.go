package remote

import (
	"errors"
	"strings"
	"testing"
)

func TestIssueCreateFailurePreservesEmptyRedactionAndByteLimit(t *testing.T) {
	if got := IssueCreateFailure(nil); got != "" {
		t.Fatalf("nil=%q", got)
	}
	if got := IssueCreateFailure(errors.New("  failure  ")); got != "failure" {
		t.Fatalf("trim=%q", got)
	}
	if got := IssueCreateFailure(errors.New("token=super-secret")); got != "<redacted>" {
		t.Fatalf("secret diagnostic=%q", got)
	}
	got := IssueCreateFailure(errors.New("https://internal.example/path " + strings.Repeat("x", 4096)))
	if strings.Contains(got, "super-secret") || strings.Contains(got, "internal.example") || len(got) != 2048 {
		t.Fatalf("redaction or bound failed: length=%d", len(got))
	}
}
