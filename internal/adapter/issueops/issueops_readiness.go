package issueops

import (
	"issueops/internal/adapter/issueops/readinesspaths"
	cycleport "issueops/internal/port/issueopscycle"
)

func ReadinessPathObservations() cycleport.ReadinessObservations {
	return cycleport.ReadinessObservations{
		WorktreePathValid:    readinesspaths.WorktreePathValid,
		PlanPathExists:       readinesspaths.PlanPathExists,
		PlanInLinkedWorktree: readinesspaths.PlanInLinkedWorktree,
		WorkspaceMatches:     samePath,
		LinkedPlanDigest:     ReviewPlanSource{}.LinkedDigest,
	}
}
