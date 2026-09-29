package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"path/filepath"
	"strings"
)

func OwnerWorkspaceLayout(record model.IssueOpsRecord) (model.OwnerWorkspaceLayout, error) {
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" {
		return model.OwnerWorkspaceLayout{}, fmt.Errorf("verified branch preparation with base_sha is required")
	}
	branch := strings.TrimSpace(record.Branch)
	leaf := strings.ReplaceAll(branch, "/", "-")
	if leaf == "" || leaf == "." || leaf == ".." {
		return model.OwnerWorkspaceLayout{}, fmt.Errorf("execution branch is invalid")
	}
	parent := strings.TrimSpace(record.BranchPrepare.ParentWorktree)
	delegated := record.Delegation != nil && strings.TrimSpace(record.Delegation.ParentCycleID) != ""
	expected := ""
	if parent != "" || delegated {
		parentLeaf := strings.ReplaceAll(strings.TrimSpace(record.BranchPrepare.BaseBranch), "/", "-")
		if parentLeaf == "" || parentLeaf == "." || parentLeaf == ".." {
			return model.OwnerWorkspaceLayout{}, fmt.Errorf("parent execution base branch is invalid")
		}
		canonical := filepath.Join(record.Repo+".worktrees", parentLeaf)
		if parent == "" {
			parent = canonical
		} else {
			parent = filepath.Clean(parent)
			expected = canonical
		}
	}
	return model.OwnerWorkspaceLayout{Root: filepath.Join(record.Repo+".worktrees", leaf), Branch: branch, BaseBranch: strings.TrimSpace(record.BranchPrepare.BaseBranch), BaseHead: strings.TrimSpace(record.BranchPrepare.BaseSHA), ParentWorktree: parent, ExpectedParent: expected}, nil
}
func ValidateOwnerWorkspaceParent(layout model.OwnerWorkspaceLayout, matches bool) error {
	if !matches {
		return fmt.Errorf("parent_worktree %q does not match canonical parent worktree %q", layout.ParentWorktree, layout.ExpectedParent)
	}
	return nil
}
