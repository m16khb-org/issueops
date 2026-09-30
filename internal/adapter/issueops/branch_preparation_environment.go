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
		return "", fmt.Errorf("git rev-parse failed: %s", strings.TrimSpace(stderr))
	}
	resolved := strings.TrimSpace(stdout)
	if resolved == "" {
		return "", fmt.Errorf("git rev-parse returned an empty commit OID")
	}
	return resolved, nil
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
