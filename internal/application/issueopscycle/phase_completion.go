package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
	cycleport "issueops/internal/port/issueopscycle"
)

func ReadinessFromMissing(record model.IssueOpsRecord, missing []string) model.IssueOpsReadiness {
	return model.IssueOpsReadiness{
		OK: true, Ready: len(missing) == 0, Missing: stringlist.UniqueSorted(missing),
		IssueURL: record.IssueURL, PlanPath: record.PlanPath, WorktreePath: record.WorktreePath, Branch: record.Branch,
	}
}

func PhaseCompletion(record model.IssueOpsRecord, phase model.IssueOpsPhase, observations cycleport.PhaseCompletionReadiness) model.IssueOpsReadiness {
	switch phase {
	case model.IssueOpsPhaseProblem:
		return ReadinessFromMissing(record, cycledomain.IntentMissing(record))
	case model.IssueOpsPhaseGrill:
		return ReadinessFromMissing(record, cycledomain.GrillReadinessMissing(record))
	case model.IssueOpsPhasePlan:
		return observations.Compatibility(record)
	case model.IssueOpsPhaseCompatibilityReview:
		return ReadinessFromMissing(record, CompatibilityReviewMissing(record))
	case model.IssueOpsPhaseImplement:
		return observations.AISlopClean(record)
	case model.IssueOpsPhaseAISlopClean:
		return ReadinessFromMissing(record, cycledomain.AISlopCleanCompletionMissing(record))
	case model.IssueOpsPhaseFeedback:
		return ReadinessFromMissing(record, FeedbackCompletionMissing(record))
	case model.IssueOpsPhasePR:
		ready := observations.PR(record)
		ready.Missing = stringlist.UniqueSorted(cycledomain.PRCompletionMissing(record, ready.Missing))
		ready.Ready = len(ready.Missing) == 0
		return ready
	case model.IssueOpsPhaseDone:
		return ReadinessFromMissing(record, cycledomain.DoneCompletionMissing(record, observations.RemoteArtifactMissing(record)))
	default:
		return model.IssueOpsReadiness{OK: true, Ready: false, Missing: []string{"unknown_phase"}}
	}
}
