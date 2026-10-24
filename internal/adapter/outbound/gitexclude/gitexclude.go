// Package gitexclude keeps harness-owned files out of `git status` through the
// repository's local <common-dir>/info/exclude, which is never committed.
package gitexclude

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/outbound/gitbin"
)

// WorkspaceStateLine excludes the runtime state issueops keeps inside a
// worktree (lease claim tokens). It is "ignored workspace state": without it a
// target repository that does not list .issueops/state/ in .gitignore shows the
// token as untracked, and every worktree_clean gate fails on harness output.
const WorkspaceStateLine = "/.issueops/state/"

// Ensure appends line to <commonDir>/info/exclude once and reports whether it
// was added now.
func Ensure(commonDir, line string) (bool, error) {
	path := filepath.Join(commonDir, "info", "exclude")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	for existing := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(existing) == line {
			return false, nil
		}
	}
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, '\n')
	}
	data = append(data, line+"\n"...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	return true, nil
}

// EnsureWorkspaceState excludes .issueops/state/ in the repository that owns
// worktree. Linked worktrees share the common dir, so one line covers all.
func EnsureWorkspaceState(worktree string) error {
	out, err := exec.Command(gitbin.Path(), "-C", worktree, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return fmt.Errorf("resolve git common dir for %s: %w", worktree, err)
	}
	_, err = Ensure(strings.TrimSpace(string(out)), WorkspaceStateLine)
	return err
}
