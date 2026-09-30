package gitworktree

import "fmt"

// ValidateRequired receives the normalized path and identity values.
func ValidateRequired(lifecycleID, source, root, branch, baseHead string) error {
	if lifecycleID == "" || source == "" || root == "" || branch == "" || baseHead == "" {
		return fmt.Errorf("lifecycle_id, source_root, root, branch, and base_head are required")
	}
	return nil
}

func ValidateLocation(source, root, parent string) error {
	if parent != source+".worktrees" {
		return fmt.Errorf("canonical worktree must use the sibling .worktrees base")
	}
	if root == source {
		return fmt.Errorf("canonical worktree must be isolated from source_root")
	}
	return nil
}

func ValidateExistingIdentity(sameRoot bool, actualBranch, branch, actualHead, baseHead string) error {
	if !sameRoot || actualBranch != branch || actualHead != baseHead {
		return fmt.Errorf("existing canonical worktree identity does not match branch and base_head")
	}
	return nil
}

// BranchCandidate is tried in order. An empty Ref needs no Git observation.
type BranchCandidate struct {
	Ref          string
	StartPoint   string
	CreateBranch bool
}

func BranchCandidates(branch, baseHead string) []BranchCandidate {
	return []BranchCandidate{
		{Ref: "refs/heads/" + branch, StartPoint: branch},
		{Ref: "refs/remotes/origin/" + branch, StartPoint: "refs/remotes/origin/" + branch, CreateBranch: true},
		{Ref: "refs/remotes/upstream/" + branch, StartPoint: "refs/remotes/upstream/" + branch, CreateBranch: true},
		{StartPoint: baseHead, CreateBranch: true},
	}
}
