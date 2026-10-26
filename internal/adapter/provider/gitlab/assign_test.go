package gitlab

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Each assignee goes with a + prefix: an unprefixed glab value replaces every
// existing assignee.
func TestGitLabAddAssigneesUsesTheAddPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "glab.log")
	writeFakeGlab(t, binDir, `#!/bin/sh
echo "$*" >> `+log+`
case "$1" in
api) echo '{"assignees":[{"username":"me"},{"username":"kim"}]}' ;;
esac
`)
	t.Setenv("PATH", binDir)
	got, err := NewProvider().AddAssignees(context.Background(), "", glMergeTestURL, "pr", []string{"kim"})
	if err != nil || strings.Join(got, ",") != "me,kim" {
		t.Fatalf("got %v, err %v", got, err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "mr update 16 -R https://gitlab.example/group/repo --assignee +kim") {
		t.Fatalf("calls = %s", calls)
	}
}
