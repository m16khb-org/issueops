package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func PlanExistenceRoot(record model.IssueOpsRecord) string {
	if worktree := strings.TrimSpace(record.WorktreePath); worktree != "" {
		return worktree
	}
	return strings.TrimSpace(record.Repo)
}

func WorktreePlanReadinessMissing(record model.IssueOpsRecord, worktreeValid, planExists, planInWorktree bool) []string {
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
