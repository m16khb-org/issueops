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

// CleanupFinishEnvironment gives cleanup finish its filesystem and git observations.
// RunGit receives the directory and git arguments and returns exit code and output.
type CleanupFinishEnvironment struct {
	RunGit func(string, ...string) (int, string)
}

// Git runs git in dir and returns the exit code and combined output.
func (e CleanupFinishEnvironment) Git(dir string, args ...string) (int, string) {
	return e.RunGit(dir, args...)
}

// Directory reports whether path is an existing directory. A missing path is false
// without error; a path that exists but is not a directory is an error.
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

// SamePath reports whether a and b are the same path after cleaning to absolute form.
func (CleanupFinishEnvironment) SamePath(a, b string) bool {
	return pathutil.CleanAbsPath(a) == pathutil.CleanAbsPath(b)
}

// PathWithin reports whether path lies inside root.
func (CleanupFinishEnvironment) PathWithin(path, root string) bool {
	return pathutil.PathWithin(path, root)
}
