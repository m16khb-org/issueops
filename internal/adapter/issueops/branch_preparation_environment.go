package issueops

import (
	"fmt"
	"path/filepath"
	"strings"

	remote "issueops/internal/domain/issueopsremote"
)

type BranchPreparationEnvironment struct {
	RunGit func(string, ...string) (int, string, string)
}

func (e BranchPreparationEnvironment) CleanParentPath(path string) (string, bool) {
	return filepath.Clean(path), filepath.IsAbs(path)
}

func (e BranchPreparationEnvironment) ResolveBaseCommit(repo, revision string) (string, error) {
	code, stdout, stderr := e.RunGit(repo, "rev-parse", "--verify", "--end-of-options", strings.TrimSpace(revision)+"^{commit}")
	if code != 0 {
		if _, any, _ := e.RunGit(repo, "rev-list", "-n", "1", "--all"); strings.TrimSpace(any) == "" {
			// Probe: a freshly initialized repository failed with only
			// "Needed a single revision".
			return "", fmt.Errorf("the repository has no commits yet; a cycle branches from an existing base commit, so make an initial commit on the base branch (and push it) first")
		}
		return "", fmt.Errorf("git rev-parse failed: %s", strings.TrimSpace(stderr))
	}
	resolved := strings.TrimSpace(stdout)
	if resolved == "" {
		return "", fmt.Errorf("git rev-parse returned an empty commit OID")
	}
	return resolved, nil
}

// IsAncestor reports whether ancestor is an ancestor of (or equal to) descendant.
func (e BranchPreparationEnvironment) IsAncestor(repo, ancestor, descendant string) (bool, error) {
	code, _, stderr := e.RunGit(repo, "merge-base", "--is-ancestor", ancestor, descendant)
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("git merge-base --is-ancestor failed: %s", strings.TrimSpace(stderr))
	}
}

func (e BranchPreparationEnvironment) ObserveCodeProjectKey(repo, provider string) (string, error) {
	if e.RunGit == nil {
		return "", fmt.Errorf("git command adapter is unavailable")
	}
	code, stdout, stderr := e.RunGit(repo, "remote", "get-url", "origin")
	if code != 0 {
		return "", fmt.Errorf("git remote get-url origin failed: %s", strings.TrimSpace(stderr))
	}
	return remote.ProjectKeyFromGitRemoteURL(strings.TrimSpace(stdout), provider)
}
