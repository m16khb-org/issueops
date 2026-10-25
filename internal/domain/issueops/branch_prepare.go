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

// BaseRerecordCommand re-records the branch preparation with a new base
// branch while keeping every sealed fact (base_sha, link verification,
// parent worktree, remote branch URL, project key). A re-prepare replaces the
// whole preparation, so an omitted flag erases that fact (QA: a re-prepare
// with only --base-branch dropped base_sha and link_verified).
func BaseRerecordCommand(record model.IssueOpsRecord, newBase string) string {
	prior := record.BranchPrepare
	if prior == nil {
		return ""
	}
	parts := []string{"issueops branch prepare --id", record.ID, "--provider", prior.Provider, "--branch", prior.Branch, "--base-branch", newBase}
	if prior.BaseSHA != "" {
		parts = append(parts, "--base-sha", prior.BaseSHA)
	}
	if prior.LinkVerified {
		parts = append(parts, "--link-verified")
	}
	for _, flag := range [][2]string{{"--parent-worktree", prior.ParentWorktree}, {"--remote-branch-url", prior.RemoteBranchURL}, {"--code-project-key", prior.CodeProjectKey}} {
		if strings.TrimSpace(flag[1]) != "" {
			parts = append(parts, flag[0], "'"+strings.ReplaceAll(flag[1], "'", `'\''`)+"'")
		}
	}
	return strings.Join(append(parts, "$RECORD_ACTOR_FLAGS --json"), " ")
}

// RequireSealedPreparationKept refuses a re-prepare after execution prepare
// that would drop the sealed base or the link verification: the worktree
// branch was cut from that base and its link was read back, and neither
// changed.
func RequireSealedPreparationKept(record model.IssueOpsRecord, req model.IssueOpsBranchPrepareRequest) error {
	prior := record.BranchPrepare
	if record.Execution == nil || prior == nil || req.Branch != prior.Branch {
		return nil
	}
	var dropped []string
	if strings.TrimSpace(prior.BaseSHA) != "" && strings.TrimSpace(req.BaseSHA) == "" {
		dropped = append(dropped, "--base-sha")
	}
	if prior.LinkVerified && !req.LinkVerified {
		dropped = append(dropped, "--link-verified")
	}
	if strings.TrimSpace(prior.ParentWorktree) != "" && strings.TrimSpace(req.ParentWorktree) == "" {
		dropped = append(dropped, "--parent-worktree")
	}
	if len(dropped) == 0 {
		return nil
	}
	return fmt.Errorf("branch prepare after execution prepare replaces the whole preparation, and this request omits %s, which would erase the sealed base or link of the existing worktree branch. Keep them: %s",
		strings.Join(dropped, ", "), BaseRerecordCommand(record, req.BaseBranch))
}
