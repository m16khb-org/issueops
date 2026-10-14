package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	statusdomain "issueops/internal/domain/issueopsstatus"
	cycleport "issueops/internal/port/issueopscycle"
)

func ApplyPhaseTransition(observations cycleport.PhaseTransitionObservations, record model.IssueOpsRecord, phase model.IssueOpsPhase) model.IssueOpsRecord {
	now := observations.Now()
	head, fingerprint := "", ""
	switch phase {
	case model.IssueOpsPhaseAISlopClean:
		head = observations.Head(record)
		fingerprint = observations.Fingerprint(record)
	case model.IssueOpsPhasePR:
		head = observations.Head(record)
	}
	return cycledomain.ApplyPhaseTransition(record, phase, now, head, fingerprint, statusdomain.ArtifactKeys(record.Phase))
}
