package issueops

import (
	"issueops/internal/adapter/issueops/readinesspaths"
	cycleport "issueops/internal/port/issueopscycle"
)

// ReadinessPathObservations returns the filesystem- and git-backed observations that the
// readiness evaluation uses for worktree path, plan file and sealed digest checks.
func ReadinessPathObservations() cycleport.ReadinessObservations {
	return cycleport.ReadinessObservations{
		WorktreePathValid:        readinesspaths.WorktreePathValid,
		PlanPathExists:           readinesspaths.PlanPathExists,
		PlanInLinkedWorktree:     readinesspaths.PlanInLinkedWorktree,
		WorkspaceMatches:         samePath,
		LinkedPlanDigest:         ReviewPlanSource{}.LinkedDigest,
		SealedArchitectureDigest: ReviewArchitectureSource{}.sealedDigest,
	}
}
