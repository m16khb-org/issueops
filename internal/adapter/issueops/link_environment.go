package issueops

import (
	"os"
	"path/filepath"

	"issueops/internal/adapter/issueops/pathutil"
	"issueops/internal/adapter/issueops/readinesspaths"
	domain "issueops/internal/domain/issueops"
)

// LinkEnvironment answers the path, plan-file and worktree questions that plan linking asks.
// It holds no state.
type LinkEnvironment struct{}

// PlanPath resolves path against worktree when it is relative and returns absolute paths
// unchanged.
func (LinkEnvironment) PlanPath(worktree, path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(worktree, path)
	}
	return path
}

// SamePlan reports whether left and right are the same path after cleaning.
func (LinkEnvironment) SamePlan(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

// PlanExists reports whether the plan file at path exists for repo.
func (LinkEnvironment) PlanExists(repo, path string) bool {
	return readinesspaths.PlanPathExists(repo, path)
}

// PlanInside reports whether the plan path lies inside the linked worktree.
func (LinkEnvironment) PlanInside(worktree, path string) bool {
	return readinesspaths.PlanPathInsideWorktree(worktree, path)
}

// ReadPlan returns the content of the plan file at path.
func (LinkEnvironment) ReadPlan(path string) (string, error) {
	raw, err := os.ReadFile(path)
	return string(raw), err
}

// WorktreeDirectory reports whether path is a valid worktree directory.
func (LinkEnvironment) WorktreeDirectory(path string) bool {
	return readinesspaths.WorktreePathValid(path)
}

// WorktreeBranch returns the branch that HEAD of the worktree at path points to.
func (LinkEnvironment) WorktreeBranch(path string) string { return pathutil.GitBranchFromHead(path) }

// ObserveWorktree describes where the worktree at path lives relative to repo: whether it
// exists, is a symlink, and sits inside the repo's sibling `<repo>.worktrees` directory, both
// before and after resolving symlinks.
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
