package commitsuggest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/testsupport"
)

// Cancelling the caller's context stops a git diff that does not finish on its
// own: Diff kills git and returns context.Canceled within 5 s.
func TestDiffReturnsWhenContextIsCancelled(t *testing.T) {
	dir := testsupport.WriteHangingCommand(t, "git")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Effects{}.Diff(ctx, root, true)
		done <- err
	}()
	if !testsupport.WaitForFile(filepath.Join(dir, "started"), 10*time.Second) {
		t.Fatal("fake git did not start within 10s")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Diff error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Diff did not return within 5s after the context was cancelled")
	}
}

// git that exits 0 while a child it started still holds stdout is a successful
// diff: WaitDelay closes the pipe after git exits, and Diff returns the output
// read so far with no error.
func TestDiffSucceedsWhenGitExitsWhileAChildHoldsStdout(t *testing.T) {
	dir := testsupport.WriteLingeringCommand(t, "git", "diff --git a/x b/x", 0)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := Effects{}.Diff(t.Context(), t.TempDir(), false)
	if err != nil || !strings.Contains(output, "diff --git a/x b/x") {
		t.Fatalf("Diff = %q, %v; want the printed diff and no error", output, err)
	}
}
