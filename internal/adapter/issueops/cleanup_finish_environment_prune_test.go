package issueops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveEmptyWorktreesParentOnlyRemovesAnEmptyWorktreesDirectory(t *testing.T) {
	root := t.TempDir()
	env := CleanupFinishEnvironment{}

	empty := filepath.Join(root, "repo.worktrees")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	env.RemoveEmptyWorktreesParent(filepath.Join(empty, "1-done"))
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Fatalf("emptied <repo>.worktrees should be removed, stat err=%v", err)
	}

	busy := filepath.Join(root, "other.worktrees")
	if err := os.MkdirAll(filepath.Join(busy, "2-live"), 0o755); err != nil {
		t.Fatal(err)
	}
	env.RemoveEmptyWorktreesParent(filepath.Join(busy, "1-done"))
	if _, err := os.Stat(filepath.Join(busy, "2-live")); err != nil {
		t.Fatalf("a sibling worktree must survive: %v", err)
	}

	plain := filepath.Join(root, "projects")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	env.RemoveEmptyWorktreesParent(filepath.Join(plain, "linked"))
	if _, err := os.Stat(plain); err != nil {
		t.Fatalf("a parent that is not <repo>.worktrees must survive even when empty: %v", err)
	}
}
