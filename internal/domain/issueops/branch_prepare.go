package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ValidatePreparationNames(branch, base string) error {
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	if err := ValidateBranch(branch); err != nil {
		return err
	}
	if base == "" {
		return fmt.Errorf("base_branch is required")
	}
	return nil
}

func ValidatePreparationParentPath(absolute bool) error {
	if !absolute {
		return fmt.Errorf("parent_worktree must be an absolute path")
	}
	return nil
}

// AdoptPreparedBranch binds a branchless cycle only after confirming its issue.
func AdoptPreparedBranch(record model.IssueOpsRecord, issueURL, branch string) (model.IssueOpsRecord, error) {
	if strings.TrimSpace(record.IssueURL) == "" {
		return model.IssueOpsRecord{}, fmt.Errorf("issue must be linked before branch prepare")
	}
	if record.IssueURL != issueURL {
		return model.IssueOpsRecord{}, fmt.Errorf("issue_url does not match linked IssueOps issue")
	}
	if strings.TrimSpace(record.Branch) == "" {
		record.Branch = branch
	} else if record.Branch != branch {
		return model.IssueOpsRecord{}, fmt.Errorf("branch does not match IssueOps record branch")
	}
	return record, nil
}

func ValidatePreparationUmbrella(umbrella model.IssueOpsRecord, branch, base string) error {
	expected := strings.TrimSpace(umbrella.Branch)
	if expected == "" || expected == strings.TrimSpace(base) {
		return nil
	}
	return fmt.Errorf("자식 작업 %s는 우산 사이클 %s의 브랜치 %s에서 분기해 그 브랜치로 합류해야 한다; "+
		"base_branch %q 대신 %s로 다시 준비하라", strings.TrimSpace(branch), umbrella.ID, expected, strings.TrimSpace(base), expected)
}

func ApplyBranchPreparation(record model.IssueOpsRecord, req model.IssueOpsBranchPrepareRequest, codeProject string, steps []model.IssueOpsBranchPrepareStep, createdAt, updatedAt string) model.IssueOpsRecord {
	record.BranchPrepare = &model.IssueOpsBranchPrepare{
		Provider: req.Provider, IssueURL: req.IssueURL, Branch: req.Branch, BaseBranch: req.BaseBranch,
		BaseSHA: req.BaseSHA, ParentWorktree: req.ParentWorktree, RemoteBranchURL: req.RemoteBranchURL,
		CodeProjectKey: codeProject, LinkVerified: req.LinkVerified, Steps: steps, CreatedAt: createdAt,
	}
	record.UpdatedAt = updatedAt
	return record
}
