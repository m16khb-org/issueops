package issueops

import (
	"fmt"
	"strings"
)

type BranchGit struct {
	Run func(string, ...string) (int, string, string)
}

func (g BranchGit) RefOID(repo, ref string) (string, bool) {
	code, out, _ := g.Run(repo, "rev-parse", "--verify", "--quiet", ref)
	return out, code == 0
}

// OriginPresent is a network observation; callers keep it outside the record lock.
func (g BranchGit) OriginPresent(repo, branch string) (bool, error) {
	code, out, stderr := g.Run(repo, "ls-remote", "--heads", "origin", "refs/heads/"+strings.TrimSpace(branch))
	if code != 0 {
		return false, fmt.Errorf("git ls-remote failed: %s", strings.TrimSpace(stderr))
	}
	return len(strings.Fields(strings.TrimSpace(out))) > 0, nil
}
