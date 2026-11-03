//go:build windows

package updatecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	adapter "issueops/internal/adapter/update"
	"issueops/internal/testsupport"
)

// On Windows the installer is the Go counterpart of install-native.sh; its
// dry run executes the canonical bin\issueops.exe inside the requested root.
func TestUpdateInstallerUsesRequestedRootAsWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", outside)
	t.Setenv("ISSUEOPS_SKIP_BUILD", "")
	testsupport.WriteFakeCommand(t, filepath.Join(root, "bin"), "issueops", "pwd -W > cwd.txt\n")
	if err := (adapter.Runtime{}).Install(root, []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "cwd.txt"))
	if err != nil {
		t.Fatalf("installer did not run in the requested root: %v", err)
	}
	expected, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := filepath.EvalSymlinks(filepath.FromSlash(strings.TrimSpace(string(raw))))
	if err != nil {
		t.Fatal(err)
	}
	if observed != expected {
		t.Fatalf("installer used ambient root: got %q, want %q", observed, expected)
	}
}
