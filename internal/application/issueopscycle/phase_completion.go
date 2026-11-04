package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
	cycleport "issueops/internal/port/issueopscycle"
)

func ReadinessFromMissing(record model.Record, missing []string) model.Readiness {
	return model.Readiness{
		OK: true, Ready: len(missing) == 0, Missing: stringlist.UniqueSorted(missing),
		IssueURL: record.IssueURL, PlanPath: record.PlanPath, WorktreePath: record.WorktreePath, Branch: record.Branch,
	}
}

func PhaseCompletion(record model.Record, phase model.Phase, observations cycleport.PhaseCompletionReadiness) model.Readiness {
	switch phase {
	case model.PhaseProblem:
		return ReadinessFromMissing(record, cycledomain.IntentMissing(record))
	case model.PhaseGrill:
		return ReadinessFromMissing(record, cycledomain.GrillReadinessMissing(record))
	case model.PhasePlan:
		return observations.Compatibility(record)
	case model.PhaseCompatibilityReview:
		return ReadinessFromMissing(record, CompatibilityReviewMissing(record))
	case model.PhaseImplement:
		return observations.AISlopClean(record)
	case model.PhaseAISlopClean:
		return ReadinessFromMissing(record, cycledomain.AISlopCleanCompletionMissing(record))
	case model.PhaseFeedback:
		return ReadinessFromMissing(record, FeedbackCompletionMissing(record))
	case model.PhasePR:
		ready := observations.PR(record)
		ready.Missing = stringlist.UniqueSorted(cycledomain.PRCompletionMissing(record, ready.Missing))
		ready.Ready = len(ready.Missing) == 0
		return ready
	case model.PhaseDone:
		return ReadinessFromMissing(record, cycledomain.DoneCompletionMissing(record, observations.RemoteArtifactMissing(record)))
	default:
		return model.Readiness{OK: true, Ready: false, Missing: []string{"unknown_phase"}}
	}
}
