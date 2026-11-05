package issueopscycle

import (
	"strings"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func compatibilityReadinessMissing(record model.Record, observations cycleport.ReadinessObservations) []string {
	missing := baseImplementationMissing(record)
	return append(missing, worktreePlanMissing(record, observations)...)
}

func implementationReadinessMissing(record model.Record, checkPlanBinding bool, observations cycleport.ReadinessObservations) []string {
	missing := compatibilityReadinessMissing(record, observations)
	missing = append(missing, CompatibilityReviewMissing(record)...)
	missing = append(missing, architectureReviewStaleMissing(record, checkPlanBinding, observations.SealedArchitectureDigest)...)
	missing = append(missing, DevilsAdvocateReviewMissing(record, checkPlanBinding, observations.LinkedPlanDigest)...)
	workspaceMatches := false
	if record.Execution != nil {
		workspaceMatches = observations.WorkspaceMatches(record.WorktreePath, record.Execution.Workspace.Root)
	}
	return append(missing, cycledomain.ExecutionReadinessMissing(record, workspaceMatches)...)
}

func worktreePlanMissing(record model.Record, observations cycleport.ReadinessObservations) []string {
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
