package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"strings"
)

// Orca creates a new branch, so a name collision must fail before provisioning.
func OrcaBranchScopes(branch string) ([]model.OrcaBranchScope, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil, fmt.Errorf("Orca prepare requires a branch name")
	}
	return []model.OrcaBranchScope{
		{Branch: branch, Ref: "refs/heads/" + branch, Where: "locally", Remedy: "delete the local branch if it holds no work"},
		{Branch: branch, Ref: "refs/remotes/origin/" + branch, Where: "on origin", Remedy: "delete the remote branch if it holds no work, or run `git fetch --prune` if it is already gone", Remote: true},
	}, nil
}

// A verified GitLab preparation may reuse exactly its sealed remote base.
func ValidateOrcaBranchRef(prepared *model.IssueOpsBranchPrepare, scope model.OrcaBranchScope, observedOID string) error {
	if scope.Remote && prepared != nil && strings.EqualFold(strings.TrimSpace(prepared.Provider), "gitlab") && prepared.LinkVerified && strings.TrimSpace(prepared.Branch) == scope.Branch && strings.TrimSpace(prepared.BaseSHA) != "" && strings.EqualFold(strings.TrimSpace(observedOID), strings.TrimSpace(prepared.BaseSHA)) {
		return nil
	}
	return fmt.Errorf("branch %q already exists %s, so Orca cannot prepare this execution: Orca always creates a new branch, so it would take a different name (observed: a numeric suffix) and fail as worktree_branch_mismatch only after the worktree exists; "+"use --mode direct with an explicit --direct-reason, which adopts the existing branch, or %s", scope.Branch, scope.Where, scope.Remedy)
}
