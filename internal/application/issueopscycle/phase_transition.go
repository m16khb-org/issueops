package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	statusdomain "issueops/internal/domain/issueopsstatus"
	cycleport "issueops/internal/port/issueopscycle"
)

func applyPhaseTransition(observations cycleport.PhaseTransitionObservations, record model.Record, phase model.Phase) model.Record {
	now := observations.Now()
	head, fingerprint := "", ""
	switch phase {
	case model.PhaseAISlopClean:
		head = observations.Head(record)
		fingerprint = observations.Fingerprint(record)
	case model.PhasePR:
		head = observations.Head(record)
	}
	return cycledomain.ApplyPhaseTransition(record, phase, now, head, fingerprint, statusdomain.ArtifactKeys(record.Phase))
}
