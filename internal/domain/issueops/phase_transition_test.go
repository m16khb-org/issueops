package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestApplyPhaseTransitionStampsCleanupAndLedger(t *testing.T) {
	record := model.Record{Phase: model.PhaseImplement}
	result := ApplyPhaseTransition(record, model.PhaseAISlopClean, "2026-09-28T00:00:00Z", "head-1", "fingerprint-1", []string{"implementation_changes"})
	if result.Phase != model.PhaseAISlopClean || result.AISlopCleanAt != "2026-09-28T00:00:00Z" ||
		result.AISlopCleanHead != "head-1" || result.AISlopCleanFingerprint != "fingerprint-1" {
		t.Fatalf("cleanup transition=%+v", result)
	}
	if result.PhaseLedger[model.PhaseImplement].CompletedAt != "2026-09-28T00:00:00Z" ||
		result.PhaseLedger[model.PhaseAISlopClean].EnteredAt != "2026-09-28T00:00:00Z" {
		t.Fatalf("ledger=%+v", result.PhaseLedger)
	}
}

func TestApplyPhaseTransitionPreservesExistingCleanupStamp(t *testing.T) {
	record := model.Record{Phase: model.PhaseImplement, AISlopCleanAt: "existing"}
	result := ApplyPhaseTransition(record, model.PhaseAISlopClean, "new", "head", "fingerprint", nil)
	if result.AISlopCleanAt != "existing" || result.AISlopCleanHead != "head" {
		t.Fatalf("cleanup transition=%+v", result)
	}
}

// A cycle is created in problem with no ledger entry; leaving problem records
// that it was entered at creation, not at the moment it was left.
func TestApplyPhaseTransitionDatesProblemFromRecordCreation(t *testing.T) {
	record := NewCycleRecord("io-1", "/repo", "", "2026-10-24T01:00:00Z")
	result := ApplyPhaseTransition(record, model.PhaseGrill, "2026-10-24T01:05:00Z", "", "", nil)
	problem := result.PhaseLedger[model.PhaseProblem]
	if problem.EnteredAt != "2026-10-24T01:00:00Z" || problem.CompletedAt != "2026-10-24T01:05:00Z" {
		t.Fatalf("problem ledger=%+v", problem)
	}
	// An already dated entry is not rewritten.
	record.PhaseLedger = model.PhaseLedger{model.PhaseProblem: {EnteredAt: "2026-10-24T01:02:00Z"}}
	if got := ApplyPhaseTransition(record, model.PhaseGrill, "2026-10-24T01:05:00Z", "", "", nil).PhaseLedger[model.PhaseProblem].EnteredAt; got != "2026-10-24T01:02:00Z" {
		t.Fatalf("existing entered_at rewritten: %s", got)
	}
}
