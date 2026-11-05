package issueops

import (
	"fmt"
	"strings"
)

// BranchGit adapts a git command runner to the branch-preparation port. Run receives the
// repository directory and git arguments and returns exit code, stdout and stderr.
type BranchGit struct {
	Run func(string, ...string) (int, string, string)
}

// OriginPresent is a network observation; callers keep it outside the record lock.
func (g BranchGit) OriginPresent(repo, branch string) (bool, error) {
	code, out, stderr := g.Run(repo, "ls-remote", "--heads", "origin", "refs/heads/"+strings.TrimSpace(branch))
	if code != 0 {
		return false, fmt.Errorf("git ls-remote failed: %s", strings.TrimSpace(stderr))
	}
	return len(strings.Fields(strings.TrimSpace(out))) > 0, nil
}
