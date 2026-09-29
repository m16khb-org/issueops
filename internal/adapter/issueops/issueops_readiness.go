package issueops

import (
	"issueops/internal/adapter/issueops/readinesspaths"
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func IssueOpsPlanReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return cycleapp.ReadinessFromMissing(record, issueopsdomain.PlanReadinessMissing(record))
}

func IssueOpsCompatibilityReviewReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return cycleapp.ReadinessFromMissing(record, cycleapp.CompatibilityReadinessMissing(record, ReadinessPathObservations()))
}

func ReadinessPathObservations() cycleport.ReadinessObservations {
	return cycleport.ReadinessObservations{
		WorktreePathValid:    issueOpsWorktreePathValid,
		PlanPathExists:       issueOpsPlanPathExists,
		PlanInLinkedWorktree: issueOpsPlanInLinkedWorktree,
		WorkspaceMatches:     samePath,
		LinkedPlanDigest:     ReviewPlanSource{}.LinkedDigest,
	}
}

func issueOpsStrictGitRoot(record issueops.IssueOpsRecord) string {
	return readinesspaths.StrictGitRoot(record)
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
