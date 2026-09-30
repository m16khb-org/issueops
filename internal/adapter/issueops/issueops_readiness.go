package issueops

import (
	"issueops/internal/adapter/issueops/readinesspaths"
	"issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func ReadinessPathObservations() cycleport.ReadinessObservations {
	return cycleport.ReadinessObservations{
		WorktreePathValid:    issueOpsWorktreePathValid,
		PlanPathExists:       issueOpsPlanPathExists,
		PlanInLinkedWorktree: issueOpsPlanInLinkedWorktree,
		WorkspaceMatches:     samePath,
		LinkedPlanDigest:     ReviewPlanSource{}.LinkedDigest,
	}
}

func issueOpsWorktreePathValid(path string) bool {
	return readinesspaths.WorktreePathValid(path)
}

func issueOpsPlanPathExists(repo, path string) bool {
	return readinesspaths.PlanPathExists(repo, path)
}

func issueOpsPlanInLinkedWorktree(record issueops.IssueOpsRecord) bool {
	return readinesspaths.PlanInLinkedWorktree(record)
}

func issueOpsPlanPathInsideWorktree(worktree, planPath string) bool {
	return readinesspaths.PlanPathInsideWorktree(worktree, planPath)
}
