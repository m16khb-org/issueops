package issueops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

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
func (CleanupFinishEnvironment) SamePath(a, b string) bool {
	return pathutil.CleanAbsPath(a) == pathutil.CleanAbsPath(b)
}
func (CleanupFinishEnvironment) PathWithin(path, root string) bool {
	return pathutil.PathWithin(path, root)
}
