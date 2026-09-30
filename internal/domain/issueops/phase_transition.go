package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func ApplyPhaseTransition(record model.IssueOpsRecord, phase model.IssueOpsPhase, now, head, fingerprint string, completedArtifacts []string) model.IssueOpsRecord {
	previous := record.Phase
	record.Phase = phase
	if phase == model.IssueOpsPhaseAISlopClean {
		if strings.TrimSpace(record.AISlopCleanAt) == "" {
			record.AISlopCleanAt = now
		}
		record.AISlopCleanHead = head
		record.AISlopCleanFingerprint = fingerprint
	}
	record.PhaseLedger = StampForwardTransition(record.PhaseLedger, previous, phase, now, completedArtifacts)
	return record
}
