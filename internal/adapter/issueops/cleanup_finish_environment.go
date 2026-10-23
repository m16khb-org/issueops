package issueops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/issueops/pathutil"
)

type CleanupFinishEnvironment struct {
	RunGit func(string, ...string) (int, string)
}

func (e CleanupFinishEnvironment) Git(dir string, args ...string) (int, string) {
	return e.RunGit(dir, args...)
}
func (CleanupFinishEnvironment) Directory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("cleanup worktree is not a directory")
	}
	return true, nil
}

// RemoveEmptyWorktreesParent removes the `<repo>.worktrees` directory that
// held a removed linked worktree once nothing else lives in it. Any other
// parent, or a non-empty one, is left alone; failures are not cleanup errors.
func (CleanupFinishEnvironment) RemoveEmptyWorktreesParent(worktreeRoot string) {
	parent := filepath.Dir(filepath.Clean(worktreeRoot))
	if !strings.HasSuffix(filepath.Base(parent), ".worktrees") {
		return
	}
	_ = os.Remove(parent) // fails with ENOTEMPTY while siblings remain
}
func (CleanupFinishEnvironment) SamePath(a, b string) bool {
	return pathutil.CleanAbsPath(a) == pathutil.CleanAbsPath(b)
}
func (CleanupFinishEnvironment) PathWithin(path, root string) bool {
	return pathutil.PathWithin(path, root)
}
