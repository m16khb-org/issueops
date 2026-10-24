package readinesspaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreePathValid(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"valid dir", dir, true},
		{"missing", "/nonexistent", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreePathValid(tt.path)
			if got != tt.expected {
				t.Errorf("WorktreePathValid(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestPlanPathExists(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(planPath, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		repo     string
		path     string
		expected bool
	}{
		{"absolute file exists", "", planPath, true},
		{"relative inside repo", dir, "plan.md", true},
		{"missing file", dir, "nope.md", false},
		{"empty path", dir, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanPathExists(tt.repo, tt.path)
			if got != tt.expected {
				t.Errorf("PlanPathExists(%q, %q) = %v, want %v", tt.repo, tt.path, got, tt.expected)
			}
		})
	}
}

func TestPlanPathInsideWorktree(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	real := filepath.Join(dir, "real")
	alias, other := filepath.Join(dir, "alias"), filepath.Join(dir, "other")
	if err := os.MkdirAll(filepath.Join(real, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{alias, other} {
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
	}
	resolvedReal, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		worktree string
		planPath string
		expected bool
	}{
		{"relative plan inside", dir, "plan.md", true},
		{"absolute subdir inside", dir, sub, true},
		{"empty path rejected", dir, "", false},
		// macOS: the worktree is recorded as /tmp/… but whoami and pwd -P give
		// /private/tmp/…; the resolved spelling of the same worktree is inside.
		{"resolved spelling of the worktree", alias, filepath.Join(resolvedReal, "sub"), true},
		// Another alias that merely resolves inside is not the worktree.
		{"other alias resolving inside", alias, filepath.Join(other, "sub"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanPathInsideWorktree(tt.worktree, tt.planPath)
			if got != tt.expected {
				t.Errorf("PlanPathInsideWorktree(%q, %q) = %v, want %v", tt.worktree, tt.planPath, got, tt.expected)
			}
		})
	}
}
