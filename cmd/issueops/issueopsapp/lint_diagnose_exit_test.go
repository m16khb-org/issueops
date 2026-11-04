package issueopsapp

import (
	"strings"
	"testing"

	"issueops/internal/testsupport"
)

// TestLintDiagnoseReturnsChildExitCodeWithoutExtraStderr pins that the root
// command, not the subcommand, ends the process with the failed command's exit
// status, and that it adds no `project: ...` line after the diagnostic prompt.
func TestLintDiagnoseReturnsChildExitCodeWithoutExtraStderr(t *testing.T) {
	repo := makeGitRepoForContract(t)
	var code int
	stderr := testsupport.CaptureStderr(t, func() error {
		code = RunRootCommand([]string{"project", "lint-diagnose", "--repo", repo, "--", "sh", "-c", "exit 7"})
		return nil
	})
	if code != 7 {
		t.Fatalf("exit code = %d, want 7; stderr:\n%s", code, stderr)
	}
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	if last := lines[len(lines)-1]; last != strings.Repeat("-", 36) {
		t.Fatalf("last stderr line = %q, want the prompt footer; stderr:\n%s", last, stderr)
	}
}
