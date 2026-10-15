package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"path/filepath"
	"strings"
)

// WorkspaceLayout은 execution prepare가 만들 canonical worktree의 위치와 신원이다.
type WorkspaceLayout struct {
	Root           string
	Branch         string
	BaseBranch     string
	BaseHead       string
	ParentWorktree string
	ExpectedParent string
}

func DeriveWorkspaceLayout(record model.IssueOpsRecord) (WorkspaceLayout, error) {
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" {
		return WorkspaceLayout{}, fmt.Errorf("verified branch preparation with base_sha is required")
	}
	branch := strings.TrimSpace(record.Branch)
	leaf := strings.ReplaceAll(branch, "/", "-")
	if leaf == "" || leaf == "." || leaf == ".." {
		return WorkspaceLayout{}, fmt.Errorf("execution branch is invalid")
	}
	parent := strings.TrimSpace(record.BranchPrepare.ParentWorktree)
	delegated := record.Delegation != nil && strings.TrimSpace(record.Delegation.ParentCycleID) != ""
	expected := ""
	if parent != "" || delegated {
		parentLeaf := strings.ReplaceAll(strings.TrimSpace(record.BranchPrepare.BaseBranch), "/", "-")
		if parentLeaf == "" || parentLeaf == "." || parentLeaf == ".." {
			return WorkspaceLayout{}, fmt.Errorf("parent execution base branch is invalid")
		}
		canonical := filepath.Join(record.Repo+".worktrees", parentLeaf)
		if parent == "" {
			parent = canonical
		} else {
			parent = filepath.Clean(parent)
			expected = canonical
		}
	}
	return WorkspaceLayout{Root: filepath.Join(record.Repo+".worktrees", leaf), Branch: branch, BaseBranch: strings.TrimSpace(record.BranchPrepare.BaseBranch), BaseHead: strings.TrimSpace(record.BranchPrepare.BaseSHA), ParentWorktree: parent, ExpectedParent: expected}, nil
}

func ValidateWorkspaceParent(layout WorkspaceLayout, matches bool) error {
	if !matches {
		return fmt.Errorf("parent_worktree %q does not match canonical parent worktree %q", layout.ParentWorktree, layout.ExpectedParent)
	}
	return nil
}
