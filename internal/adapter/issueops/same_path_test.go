package issueops

import (
	"os"
	"path/filepath"
	"testing"
)

// A deleted worktree keeps its two spellings comparable: git lists it under
// the symlink-resolved path while the record keeps the lexical one.
func TestSamePathComparesMissingPathsThroughResolvedParents(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	missingViaLink := filepath.Join(link, "repo.worktrees", "1-x")
	missingResolved := filepath.Join(resolved, "repo.worktrees", "1-x")
	if !samePath(missingViaLink, missingResolved) {
		t.Fatalf("missing path through a symlink must match its resolved spelling: %q vs %q", missingViaLink, missingResolved)
	}
	if samePath(missingViaLink, filepath.Join(resolved, "repo.worktrees", "2-y")) {
		t.Fatal("different missing paths must not match")
	}
}
