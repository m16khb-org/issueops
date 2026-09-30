package issueopspreparation

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type ResumeWorkspacePlan struct {
	Request        preparationcontract.WorkspaceRequest
	ExpectedParent string
}

func ResumeWorkspace(record preparationcontract.Record) (ResumeWorkspacePlan, error) {
	var prepared struct {
		BaseBranch     string `json:"base_branch"`
		BaseSHA        string `json:"base_sha"`
		ParentWorktree string `json:"parent_worktree"`
	}
	if json.Unmarshal(record.BranchPrepare, &prepared) != nil || strings.TrimSpace(prepared.BaseSHA) == "" {
		return ResumeWorkspacePlan{}, fmt.Errorf("verified branch preparation with base_sha is required")
	}
	branch := strings.TrimSpace(record.Branch)
	leaf := strings.ReplaceAll(branch, "/", "-")
	if leaf == "" || leaf == "." || leaf == ".." {
		return ResumeWorkspacePlan{}, fmt.Errorf("execution branch is invalid")
	}
	parentWorktree := strings.TrimSpace(prepared.ParentWorktree)
	var delegation struct {
		ParentCycleID string `json:"parent_cycle_id"`
	}
	_ = json.Unmarshal(record.Delegation, &delegation)
	plan := ResumeWorkspacePlan{}
	if parentWorktree != "" || strings.TrimSpace(delegation.ParentCycleID) != "" {
		parentLeaf := strings.ReplaceAll(strings.TrimSpace(prepared.BaseBranch), "/", "-")
		if parentLeaf == "" || parentLeaf == "." || parentLeaf == ".." {
			return ResumeWorkspacePlan{}, fmt.Errorf("parent execution base branch is invalid")
		}
		plan.ExpectedParent = filepath.Join(record.Repo+".worktrees", parentLeaf)
		if parentWorktree == "" {
			parentWorktree = plan.ExpectedParent
		} else {
			parentWorktree = filepath.Clean(parentWorktree)
		}
	}
	plan.Request = preparationcontract.WorkspaceRequest{
		LifecycleID: record.ID, SourceRoot: record.Repo, Root: filepath.Join(record.Repo+".worktrees", leaf),
		Branch: branch, BaseBranch: strings.TrimSpace(prepared.BaseBranch),
		BaseHead: strings.TrimSpace(prepared.BaseSHA), ParentWorktree: parentWorktree, Confirm: true,
	}
	return plan, nil
}
