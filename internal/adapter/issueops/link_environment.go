package issueops

import (
	"os"
	"path/filepath"

	"issueops/internal/adapter/issueops/pathutil"
	"issueops/internal/adapter/issueops/readinesspaths"
	domain "issueops/internal/domain/issueops"
)

type LinkEnvironment struct{}

func (LinkEnvironment) PlanPath(worktree, path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(worktree, path)
	}
	return path
}
func (LinkEnvironment) SamePlan(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
func (LinkEnvironment) PlanExists(repo, path string) bool {
	return readinesspaths.PlanPathExists(repo, path)
}
func (LinkEnvironment) PlanInside(worktree, path string) bool {
	return readinesspaths.PlanPathInsideWorktree(worktree, path)
}
func (LinkEnvironment) ReadPlan(path string) (string, error) {
	raw, err := os.ReadFile(path)
	return string(raw), err
}
func (LinkEnvironment) WorktreeDirectory(path string) bool {
	return readinesspaths.WorktreePathValid(path)
}
func (LinkEnvironment) WorktreeBranch(path string) string { return pathutil.GitBranchFromHead(path) }
func (LinkEnvironment) ObserveWorktree(repo, path string) domain.WorktreeLocation {
	location := domain.WorktreeLocation{Repo: pathutil.CleanAbsPath(repo), Path: pathutil.CleanAbsPath(path)}
	location.Parent = filepath.Join(filepath.Dir(location.Repo), filepath.Base(location.Repo)+".worktrees")
	location.InsideParent = pathutil.PathWithin(location.Path, location.Parent)
	info, err := os.Lstat(location.Path)
	location.Exists = err == nil
	if err == nil {
		location.Symlink = info.Mode()&os.ModeSymlink != 0
	}
	location.ResolvedRepo, err = filepath.EvalSymlinks(location.Repo)
	location.RepoResolved = err == nil
	location.ResolvedPath, err = filepath.EvalSymlinks(location.Path)
	location.PathResolved = err == nil
	location.ResolvedParent = filepath.Join(filepath.Dir(location.ResolvedRepo), filepath.Base(location.ResolvedRepo)+".worktrees")
	location.InsideResolvedParent = pathutil.PathWithin(location.ResolvedPath, location.ResolvedParent)
	return location
}
