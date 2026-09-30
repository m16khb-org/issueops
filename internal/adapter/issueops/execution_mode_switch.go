package issueops

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type ModeSwitchWorkspace struct {
	Git func(string, ...string) (int, string)
}

func (w ModeSwitchWorkspace) Present(root string) bool { _, err := os.Stat(root); return err == nil }
func (w ModeSwitchWorkspace) Clean(root string) bool {
	code, out := w.Git(root, "status", "--porcelain=v1")
	return code == 0 && strings.TrimSpace(out) == ""
}
func (w ModeSwitchWorkspace) CommitCount(root, ref string) (string, bool) {
	code, out := w.Git(root, "rev-list", "--count", ref+"..HEAD")
	return strings.TrimSpace(out), code == 0
}
func (w ModeSwitchWorkspace) BranchOID(repo, branch string, remote bool) (string, bool) {
	prefix := "refs/heads/"
	if remote {
		prefix = "refs/remotes/origin/"
	}
	code, out := w.Git(repo, "rev-parse", "--verify", "--quiet", prefix+branch)
	return strings.TrimSpace(out), code == 0
}
func (w ModeSwitchWorkspace) RemoveWorktree(_ context.Context, repo, root string) error {
	if code, out := w.Git(repo, "worktree", "remove", "--force", root); code != 0 {
		return fmt.Errorf("switch-mode could not remove the canonical worktree (record preserved): %s", strings.TrimSpace(out))
	}
	return nil
}
func (w ModeSwitchWorkspace) RemoveBranch(_ context.Context, repo, branch string) error {
	if code, out := w.Git(repo, "branch", "-D", branch); code != 0 {
		return fmt.Errorf("switch-mode could not remove the local branch (record preserved): %s", strings.TrimSpace(out))
	}
	return nil
}
