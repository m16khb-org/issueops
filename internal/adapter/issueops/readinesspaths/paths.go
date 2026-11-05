// Package readinesspaths holds the filesystem checks behind IssueOps
// readiness: where the git root and worktree of a cycle record live and whether
// its plan file exists inside that worktree. It is an adapter-layer helper over
// the os package.
package readinesspaths

import (
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/issueops/pathutil"
	model "issueops/internal/contract/issueops"
)

// StrictGitRoot returns the directory readiness checks run git in: the
// record's worktree path when set, otherwise its repo, trimmed of whitespace.
func StrictGitRoot(record model.Record) string {
	if path := strings.TrimSpace(record.WorktreePath); path != "" {
		return path
	}
	return strings.TrimSpace(record.Repo)
}

// WorktreePathValid reports whether path names an existing directory. Empty
// paths and paths containing a NUL byte are invalid.
func WorktreePathValid(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// PlanPathExists reports whether path, resolved against repo when relative, is
// an existing regular file (not a directory).
func PlanPathExists(repo, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(strings.TrimSpace(repo), path)
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// PlanInLinkedWorktree reports whether the record's plan path stays inside its
// linked worktree. A record with no plan path or no worktree passes, since there
// is nothing to compare.
func PlanInLinkedWorktree(record model.Record) bool {
	planPath := strings.TrimSpace(record.PlanPath)
	worktree := strings.TrimSpace(record.WorktreePath)
	if planPath == "" || worktree == "" {
		return true
	}
	return PlanPathInsideWorktree(worktree, planPath)
}

// PlanPathInsideWorktree reports whether planPath lies inside worktree. A
// relative plan path always passes. An absolute one must sit under the worktree
// as recorded or as symlink-resolved, and its own symlink-resolved target must
// stay inside the resolved worktree.
func PlanPathInsideWorktree(worktree, planPath string) bool {
	planPath = strings.TrimSpace(planPath)
	if planPath == "" || strings.Contains(planPath, "\x00") {
		return false
	}
	if !filepath.IsAbs(planPath) {
		return true
	}
	resolvedWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return false
	}
	// The plan path is written under the worktree as recorded or under its
	// resolved spelling (macOS reports /tmp as /private/tmp to whoami and
	// pwd -P); any other alias that only resolves inside is still refused.
	if !pathutil.PathWithin(planPath, worktree) && !pathutil.PathWithin(planPath, resolvedWorktree) {
		return false
	}
	resolvedPlan, err := filepath.EvalSymlinks(planPath)
	if err != nil {
		return false
	}
	return pathutil.PathWithin(resolvedPlan, resolvedWorktree)
}
