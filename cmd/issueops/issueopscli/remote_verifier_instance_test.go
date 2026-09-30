package issueopscli

import (
	"errors"
	"testing"
)

// A later command must not replace a previously composed verifier. A refusal
// must reach the caller before attempting a local link with the missing ID.
func TestChildVerificationRemainsBoundToCommand(t *testing.T) {
	firstErr, secondErr := errors.New("first provider unavailable"), errors.New("second provider unavailable")
	first := testIssueOpsCommand()
	first.VerifyChild = func(string) error { return firstErr }
	second := testIssueOpsCommand()
	second.VerifyChild = func(string) error { return secondErr }
	for _, tc := range []struct {
		cli  command
		want error
	}{{first, firstErr}, {second, secondErr}, {first, firstErr}} {
		_, err := captureStdoutAndErrorForIssueOps(t, func() error {
			return tc.cli.runIssueOpsLinkChild([]string{"--id", "missing", "--child-url", "https://github.com/acme/repo/issues/9", "--json"})
		})
		if !errors.Is(err, tc.want) {
			t.Fatalf("command verifier error = %v, want %v", err, tc.want)
		}
	}
}
