package gitlab

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Renaming a draft MR keeps its Draft: prefix (clearing it is mark-ready),
// and the returned title has the prefix removed.
func TestGitLabRetitleKeepsTheDraftPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "glab.log")
	writeFakeGlab(t, binDir, `#!/bin/sh
echo "$*" >> `+log+`
case "$*" in
*"--method PUT"*) echo '{"title":"Draft: feat: new name"}'; exit 0 ;;
esac
echo '{"web_url":"`+glMergeTestURL+`","title":"Draft: old","state":"opened","draft":true,"sha":"abc"}'
`)
	t.Setenv("PATH", binDir)
	got, err := NewProvider().Retitle(context.Background(), "", glMergeTestURL, "pr", "feat: new name")
	if err != nil || got != "feat: new name" {
		t.Fatalf("got %q, err %v", got, err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "--method PUT -f title=Draft: feat: new name") {
		t.Fatalf("calls = %s", calls)
	}
}
