package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ValidateWorkspaceLinkPath(field, path string) error {
	if path == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return fmt.Errorf("%s must not contain path traversal", field)
	}
	return nil
}
func ValidateLinkBranchEvidence(record model.IssueOpsRecord, kind string) error {
	if missing := BranchEvidenceMissing(record); len(missing) > 0 {
		return fmt.Errorf("cannot link %s before branch evidence: missing %s", kind, strings.Join(missing, ", "))
	}
	return nil
}
func ValidatePlanLinkReadiness(record model.IssueOpsRecord, designMissing []string) error {
	if err := ValidateLinkBranchEvidence(record, "plan"); err != nil {
		return err
	}
	if strings.TrimSpace(record.WorktreePath) == "" {
		return fmt.Errorf("cannot link plan before linked worktree")
	}
	if len(designMissing) > 0 {
		return fmt.Errorf("cannot link plan before approved design review: missing %s", strings.Join(designMissing, ", "))
	}
	return nil
}
func ValidatePlanLinkLocation(path, worktree string, exists, inside bool) error {
	if !exists {
		return fmt.Errorf("plan_path does not exist: %s", path)
	}
	return ValidateLinkedPlanContainment(worktree, inside)
}
func ValidateLinkedPlanContainment(worktree string, inside bool) error {
	if !inside {
		return fmt.Errorf("plan_path must be inside linked worktree: %s", worktree)
	}
	return nil
}
func PlanLinkIsUnchanged(linked string, samePath bool) (bool, error) {
	if strings.TrimSpace(linked) == "" {
		return false, nil
	}
	if samePath {
		return true, nil
	}
	return false, fmt.Errorf("plan_path is already linked; edit the linked plan in place instead of replacing its identity")
}
func ValidateLinkedPlanSections(missing []string) error {
	if len(missing) > 0 {
		return fmt.Errorf("plan is missing required sections: %s", strings.Join(missing, ", "))
	}
	return nil
}
func ApplyPlanLink(record model.IssueOpsRecord, path, now string) model.IssueOpsRecord {
	record.PlanPath = path
	record.UpdatedAt = now
	return record
}
func ApplyWorktreeLink(record model.IssueOpsRecord, path, now string) model.IssueOpsRecord {
	record.WorktreePath = path
	record.UpdatedAt = now
	return record
}
func ValidateWorktreeDirectory(path string, directory bool) error {
	if !directory {
		return fmt.Errorf("worktree_path does not exist or is not a directory: %s", path)
	}
	return nil
}

type WorktreeLocation struct {
	Repo, Path, Parent, ResolvedRepo, ResolvedPath, ResolvedParent                  string
	InsideParent, Exists, Symlink, RepoResolved, PathResolved, InsideResolvedParent bool
}

func ValidateWorktreeLocation(location WorktreeLocation) error {
	if location.Repo == "" || location.Path == "" {
		return fmt.Errorf("worktree_path and repo must be absolute or resolvable paths")
	}
	if location.Path == location.Repo {
		return fmt.Errorf("worktree_path must be isolated from the source checkout")
	}
	if !location.InsideParent {
		return fmt.Errorf("worktree_path must be under sibling worktree directory: %s", location.Parent)
	}
	if !location.Exists {
		return fmt.Errorf("worktree_path does not exist: %s", location.Path)
	}
	if location.Symlink {
		return fmt.Errorf("worktree_path must not be a symlink: %s", location.Path)
	}
	if !location.RepoResolved {
		return fmt.Errorf("source checkout path cannot be resolved: %s", location.Repo)
	}
	if !location.PathResolved {
		return fmt.Errorf("worktree_path cannot be resolved: %s", location.Path)
	}
	if location.ResolvedPath == location.ResolvedRepo {
		return fmt.Errorf("worktree_path must be isolated from the source checkout")
	}
	if !location.InsideResolvedParent {
		return fmt.Errorf("worktree_path must resolve under sibling worktree directory: %s", location.ResolvedParent)
	}
	return nil
}
func ValidateLinkedWorktreeBranch(expected, actual string) error {
	expected, actual = strings.TrimSpace(expected), strings.TrimSpace(actual)
	if expected == "" {
		return nil
	}
	if actual == "" {
		return fmt.Errorf("worktree_path must be a git worktree on IssueOps branch %s", expected)
	}
	if actual != expected {
		return fmt.Errorf("worktree branch %s does not match IssueOps branch %s", actual, expected)
	}
	return nil
}
