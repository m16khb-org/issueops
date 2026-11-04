package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// PlanExistenceRoot returns the directory a plan path is resolved against:
// the linked worktree, or the repository when no worktree is linked.
func PlanExistenceRoot(record model.Record) string {
	if worktree := strings.TrimSpace(record.WorktreePath); worktree != "" {
		return worktree
	}
	return strings.TrimSpace(record.Repo)
}

// WorktreePlanReadinessMissing lists the unmet worktree and plan gates: a
// linked and valid worktree, an existing plan when one is linked, and a plan
// inside the worktree. The caller supplies the filesystem observations.
func WorktreePlanReadinessMissing(record model.Record, worktreeValid, planExists, planInWorktree bool) []string {
	missing := []string{}
	if strings.TrimSpace(record.WorktreePath) == "" {
		missing = append(missing, "worktree_path")
	} else if !worktreeValid {
		missing = append(missing, "worktree_exists")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !planExists {
		missing = append(missing, "plan_exists")
	}
	if !planInWorktree {
		missing = append(missing, "plan_in_worktree")
	}
	return missing
}
