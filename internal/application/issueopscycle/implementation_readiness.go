package issueopscycle

import (
	"strings"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func CompatibilityReadinessMissing(record model.IssueOpsRecord, observations cycleport.ReadinessObservations) []string {
	missing := BaseImplementationMissing(record)
	return append(missing, worktreePlanMissing(record, observations)...)
}

func ImplementationReadinessMissing(record model.IssueOpsRecord, checkPlanBinding bool, observations cycleport.ReadinessObservations) []string {
	missing := CompatibilityReadinessMissing(record, observations)
	missing = append(missing, CompatibilityReviewMissing(record)...)
	missing = append(missing, DevilsAdvocateReviewMissing(record, checkPlanBinding, observations.LinkedPlanDigest)...)
	workspaceMatches := false
	if record.Execution != nil {
		workspaceMatches = observations.WorkspaceMatches(record.WorktreePath, record.Execution.Workspace.Root)
	}
	return append(missing, cycledomain.ExecutionReadinessMissing(record, workspaceMatches)...)
}

func worktreePlanMissing(record model.IssueOpsRecord, observations cycleport.ReadinessObservations) []string {
	worktreeValid := false
	if path := strings.TrimSpace(record.WorktreePath); path != "" {
		worktreeValid = observations.WorktreePathValid(path)
	}
	planExists := false
	if strings.TrimSpace(record.PlanPath) != "" {
		planExists = observations.PlanPathExists(cycledomain.PlanExistenceRoot(record), record.PlanPath)
	}
	planInWorktree := observations.PlanInLinkedWorktree(record)
	return cycledomain.WorktreePlanReadinessMissing(record, worktreeValid, planExists, planInWorktree)
}
