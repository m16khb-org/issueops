package issueops

import (
	"fmt"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/issueops/active"
	model "issueops/internal/contract/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

type BranchPreparationEnvironment struct{}

func (BranchPreparationEnvironment) CleanParentPath(path string) (string, bool) {
	return filepath.Clean(path), filepath.IsAbs(path)
}

func (BranchPreparationEnvironment) ResolveBaseCommit(repo, revision string) (string, error) {
	code, stdout, stderr := GitCmd(repo, "rev-parse", "--verify", "--end-of-options", strings.TrimSpace(revision)+"^{commit}")
	if code != 0 {
		return "", fmt.Errorf("git rev-parse failed: %s", strings.TrimSpace(stderr))
	}
	resolved := strings.TrimSpace(stdout)
	if resolved == "" {
		return "", fmt.Errorf("git rev-parse returned an empty commit OID")
	}
	return resolved, nil
}

func (BranchPreparationEnvironment) UmbrellaForChildIssue(repo, childIssueURL string) (model.IssueOpsRecord, bool) {
	return active.UmbrellaCycleForChildIssue(issueOpsActiveStore(), repo, childIssueURL)
}

func (BranchPreparationEnvironment) ObserveCodeProjectKey(repo, provider string) (string, error) {
	if GitCmd == nil {
		return "", fmt.Errorf("git command adapter is unavailable")
	}
	code, stdout, stderr := GitCmd(repo, "remote", "get-url", "origin")
	if code != 0 {
		return "", fmt.Errorf("git remote get-url origin failed: %s", strings.TrimSpace(stderr))
	}
	return remote.ProjectKeyFromGitRemoteURL(strings.TrimSpace(stdout), provider)
}
