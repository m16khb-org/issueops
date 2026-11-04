// Package pathutil provides filesystem path and git state helpers for the
// issueops adapters. They read files directly and never run git.
package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// CleanAbsPath trims path, makes it absolute against the working directory, and
// cleans it. A blank path returns "". If the working directory cannot be
// resolved the cleaned relative path is returned.
func CleanAbsPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return filepath.Clean(p)
		}
		p = abs
	}
	return filepath.Clean(p)
}

// PathWithin reports whether path equals root or lies beneath it, after both
// are made absolute and cleaned. Blank inputs never match.
func PathWithin(path, root string) bool {
	p := CleanAbsPath(path)
	r := CleanAbsPath(root)
	if p == "" || r == "" {
		return false
	}
	if p == r {
		return true
	}
	rel, err := filepath.Rel(r, p)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

// gitDir resolves a checkout's git directory without running git: .git
// itself, or the directory a linked worktree's .git file points to.
func gitDir(repo string) string {
	root := CleanAbsPath(repo)
	if root == "" {
		return ""
	}
	gitPath := filepath.Join(root, ".git")
	if info, err := os.Stat(gitPath); err == nil && !info.IsDir() {
		if b, err := os.ReadFile(gitPath); err == nil {
			line := strings.TrimSpace(string(b))
			if rest, ok := strings.CutPrefix(line, "gitdir:"); ok {
				resolved := strings.TrimSpace(rest)
				if !filepath.IsAbs(resolved) {
					resolved = filepath.Join(root, resolved)
				}
				return resolved
			}
		}
	}
	return gitPath
}

// GitOperationInProgress names an unfinished merge, rebase, cherry-pick or
// revert in a checkout from the state files git leaves behind, or "".
func GitOperationInProgress(repo string) string {
	dir := gitDir(repo)
	if dir == "" {
		return ""
	}
	for _, state := range []struct{ file, operation string }{
		{"MERGE_HEAD", "merge"}, {"rebase-merge", "rebase"}, {"rebase-apply", "rebase"},
		{"CHERRY_PICK_HEAD", "cherry-pick"}, {"REVERT_HEAD", "revert"},
	} {
		if _, err := os.Stat(filepath.Join(dir, state.file)); err == nil {
			return state.operation
		}
	}
	return ""
}

// GitBranchFromHead returns the branch a checkout has checked out, read from
// its HEAD file without running git. A detached HEAD or unreadable repo returns "".
func GitBranchFromHead(repo string) string {
	dir := gitDir(repo)
	if dir == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(b))
	if rest, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		return strings.TrimSpace(rest)
	}
	return ""
}
