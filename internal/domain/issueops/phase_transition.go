package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func ApplyPhaseTransition(record model.Record, phase model.Phase, now, head, fingerprint string, completedArtifacts []string) model.Record {
	previous := record.Phase
	record.Phase = phase
	if phase == model.PhaseAISlopClean {
		if strings.TrimSpace(record.AISlopCleanAt) == "" {
			record.AISlopCleanAt = now
		}
		record.AISlopCleanHead = head
		record.AISlopCleanFingerprint = fingerprint
	}
	if phase == model.PhasePR {
		record.PRHead = head
	}
	// A cycle starts in problem without a ledger entry; it was entered when the
	// record was created, not when it is left.
	if previous == model.PhaseProblem && record.PhaseLedger[previous].EnteredAt == "" && record.CreatedAt != "" {
		if record.PhaseLedger == nil {
			record.PhaseLedger = model.PhaseLedger{}
		}
		entry := record.PhaseLedger[previous]
		entry.EnteredAt = record.CreatedAt
		record.PhaseLedger[previous] = entry
	}
	record.PhaseLedger = StampForwardTransition(record.PhaseLedger, previous, phase, now, completedArtifacts)
	return record
}
