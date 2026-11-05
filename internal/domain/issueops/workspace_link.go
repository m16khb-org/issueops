package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ValidateWorkspaceLinkPath requires a non-empty link path without a NUL byte
// or ".." traversal. field names the flag in the error.
func ValidateWorkspaceLinkPath(field, path string) error {
	if path == "" {
		return fmt.Errorf("%s is required (--%s)", field, strings.ReplaceAll(field, "_", "-"))
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return fmt.Errorf("%s must not contain path traversal", field)
	}
	return nil
}

// ValidateLinkBranchEvidence requires the branch evidence (issue URL, branch,
// prepared and link-verified branch) before linking kind, such as a plan or
// worktree.
func ValidateLinkBranchEvidence(record model.Record, kind string) error {
	if missing := BranchEvidenceMissing(record); len(missing) > 0 {
		return fmt.Errorf("cannot link %s before branch evidence: missing %s", kind, strings.Join(missing, ", "))
	}
	return nil
}

// ValidatePlanLinkReadiness checks that a plan can be linked: branch evidence
// exists, a worktree is linked, and architectureMissing, the architecture
// review gaps found by the caller, is empty.
func ValidatePlanLinkReadiness(record model.Record, architectureMissing []string) error {
	if err := ValidateLinkBranchEvidence(record, "plan"); err != nil {
		return err
	}
	if strings.TrimSpace(record.WorktreePath) == "" {
		return fmt.Errorf("cannot link plan before linked worktree")
	}
	if len(architectureMissing) > 0 {
		return fmt.Errorf("cannot link plan before approved architecture review: missing %s", strings.Join(architectureMissing, ", "))
	}
	return nil
}

// ValidatePlanLinkLocation requires the plan file to exist and to lie inside
// the linked worktree; the caller supplies both observations.
func ValidatePlanLinkLocation(path, worktree string, exists, inside bool) error {
	if !exists {
		return fmt.Errorf("plan_path does not exist: %s", path)
	}
	return ValidateLinkedPlanContainment(worktree, inside)
}

// ValidateLinkedPlanContainment fails when the plan path is not inside the
// linked worktree.
func ValidateLinkedPlanContainment(worktree string, inside bool) error {
	if !inside {
		return fmt.Errorf("plan_path must be inside linked worktree: %s", worktree)
	}
	return nil
}

// PlanLinkIsUnchanged reports whether linking the plan again is a no-op. It
// is when the same path is already linked; a different path is an error
// because a linked plan is edited in place; with no linked plan it returns
// false.
func PlanLinkIsUnchanged(linked string, samePath bool) (bool, error) {
	if strings.TrimSpace(linked) == "" {
		return false, nil
	}
	if samePath {
		return true, nil
	}
	return false, fmt.Errorf("plan_path is already linked; edit the linked plan in place instead of replacing its identity")
}

// ValidateLinkedPlanSections fails when the plan lacks any required section,
// listing the missing headings.
func ValidateLinkedPlanSections(missing []string) error {
	if len(missing) > 0 {
		return fmt.Errorf("plan is missing required sections: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ApplyPlanLink records path as the plan of the record and stamps now as the
// update time.
func ApplyPlanLink(record model.Record, path, now string) model.Record {
	record.PlanPath = path
	record.UpdatedAt = now
	return record
}

// ApplyWorktreeLink records path as the worktree of the record and stamps now
// as the update time.
func ApplyWorktreeLink(record model.Record, path, now string) model.Record {
	record.WorktreePath = path
	record.UpdatedAt = now
	return record
}

// ValidateWorktreeDirectory fails when the worktree path is not an existing
// directory; the caller supplies that observation.
func ValidateWorktreeDirectory(path string, directory bool) error {
	if !directory {
		return fmt.Errorf("worktree_path does not exist or is not a directory: %s", path)
	}
	return nil
}

// WorktreeLocation carries the path observations needed to validate a
// worktree link: the repository, worktree, and parent paths in raw and
// symlink-resolved form, and whether each exists, is a symlink, resolves, and
// lies under the sibling worktree directory. The caller gathers them; the
// domain only judges them.
type WorktreeLocation struct {
	Repo, Path, Parent, ResolvedRepo, ResolvedPath, ResolvedParent                  string
	InsideParent, Exists, Symlink, RepoResolved, PathResolved, InsideResolvedParent bool
}

// ValidateWorktreeLocation checks that the worktree is isolated from the
// source checkout: a distinct path under the sibling worktree directory,
// existing, not a symlink, and still under that directory after resolution.
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

// ValidateLinkedWorktreeBranch requires the worktree to be checked out on the
// expected IssueOps branch. A blank expected branch accepts anything.
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
