package issueopsremote

import (
	"errors"
	"strings"
	"testing"
)

func TestDurableIssueCreateFailureRedactsAndCapsDiagnostics(t *testing.T) {
	got := IssueCreateFailure(errors.New(
		"token=super-secret https://internal.example/path " + strings.Repeat("x", 4096),
	))

	if strings.Contains(got, "super-secret") || strings.Contains(got, "internal.example") {
		t.Fatalf("durable diagnostic was not redacted: %q", got)
	}
	if len(got) > 2048 {
		t.Fatalf("durable diagnostic length = %d, want <= 2048", len(got))
	}
}
