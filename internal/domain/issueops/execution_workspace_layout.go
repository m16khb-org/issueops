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

func DeriveWorkspaceLayout(record model.Record) (WorkspaceLayout, error) {
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" {
		return WorkspaceLayout{}, fmt.Errorf("verified branch preparation with base_sha is required")
	}
	branch := strings.TrimSpace(record.Branch)
	leaf := strings.ReplaceAll(branch, "/", "-")
	if invalidWorktreeLeaf(leaf) {
		return WorkspaceLayout{}, fmt.Errorf("execution branch is invalid")
	}
	parent, expected, err := deriveParentWorktree(record, strings.TrimSpace(record.BranchPrepare.ParentWorktree))
	if err != nil {
		return WorkspaceLayout{}, err
	}
	return WorkspaceLayout{Root: filepath.Join(record.Repo+".worktrees", leaf), Branch: branch, BaseBranch: strings.TrimSpace(record.BranchPrepare.BaseBranch), BaseHead: strings.TrimSpace(record.BranchPrepare.BaseSHA), ParentWorktree: parent, ExpectedParent: expected}, nil
}

// invalidWorktreeLeaf는 worktree 디렉터리 이름으로 쓸 수 없는 값인지 판단한다.
func invalidWorktreeLeaf(leaf string) bool {
	return leaf == "" || leaf == "." || leaf == ".."
}

// deriveParentWorktree는 상위 worktree 경로와 canonical 기대 경로를 정한다. 상위가 없고 위임도 아니면 둘 다 비어 있다.
func deriveParentWorktree(record model.Record, parent string) (string, string, error) {
	delegated := record.Delegation != nil && strings.TrimSpace(record.Delegation.ParentCycleID) != ""
	if parent == "" && !delegated {
		return parent, "", nil
	}
	parentLeaf := strings.ReplaceAll(strings.TrimSpace(record.BranchPrepare.BaseBranch), "/", "-")
	if invalidWorktreeLeaf(parentLeaf) {
		return "", "", fmt.Errorf("parent execution base branch is invalid")
	}
	canonical := filepath.Join(record.Repo+".worktrees", parentLeaf)
	if parent == "" {
		return canonical, "", nil
	}
	return filepath.Clean(parent), canonical, nil
}

func ValidateWorkspaceParent(layout WorkspaceLayout, matches bool) error {
	if !matches {
		return fmt.Errorf("parent_worktree %q does not match canonical parent worktree %q", layout.ParentWorktree, layout.ExpectedParent)
	}
	return nil
}
