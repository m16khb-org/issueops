package gitraw

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportsOutputByteForByte(t *testing.T) {
	dir := t.TempDir()
	if code, _, stderr := Run(dir, "init", "--quiet"); code != 0 {
		t.Fatalf("init code=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := Run(dir, "rev-parse", "--show-cdup")
	if code != 0 || stdout != "\n" || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q; want the untrimmed newline", code, stdout, stderr)
	}
}

func TestRunReportsGitExitCodeAndStderr(t *testing.T) {
	dir := t.TempDir()
	code, stdout, stderr := Run(dir, "rev-parse", "--verify", "refs/heads/does-not-exist")
	if code == 0 || stdout != "" || stderr == "" {
		t.Fatalf("code=%d stdout=%q stderr=%q; want a non-zero git exit with its message", code, stdout, stderr)
	}
}

func TestRunReportsLaunchFailureAsExitCodeOneWithError(t *testing.T) {
	code, _, stderr := Run(filepath.Join(t.TempDir(), "missing-dir"), "status")
	if code != 1 || strings.TrimSpace(stderr) == "" {
		t.Fatalf("code=%d stderr=%q; want code 1 with the launch error", code, stderr)
	}
}
