package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestApplyPhaseTransitionStampsCleanupAndLedger(t *testing.T) {
	record := model.IssueOpsRecord{Phase: model.IssueOpsPhaseImplement}
	result := ApplyPhaseTransition(record, model.IssueOpsPhaseAISlopClean, "2026-09-28T00:00:00Z", "head-1", "fingerprint-1", []string{"implementation_changes"})
	if result.Phase != model.IssueOpsPhaseAISlopClean || result.AISlopCleanAt != "2026-09-28T00:00:00Z" ||
		result.AISlopCleanHead != "head-1" || result.AISlopCleanFingerprint != "fingerprint-1" {
		t.Fatalf("cleanup transition=%+v", result)
	}
	if result.PhaseLedger[model.IssueOpsPhaseImplement].CompletedAt != "2026-09-28T00:00:00Z" ||
		result.PhaseLedger[model.IssueOpsPhaseAISlopClean].EnteredAt != "2026-09-28T00:00:00Z" {
		t.Fatalf("ledger=%+v", result.PhaseLedger)
	}
}

func TestApplyPhaseTransitionPreservesExistingCleanupStamp(t *testing.T) {
	record := model.IssueOpsRecord{Phase: model.IssueOpsPhaseImplement, AISlopCleanAt: "existing"}
	result := ApplyPhaseTransition(record, model.IssueOpsPhaseAISlopClean, "new", "head", "fingerprint", nil)
	if result.AISlopCleanAt != "existing" || result.AISlopCleanHead != "head" {
		t.Fatalf("cleanup transition=%+v", result)
	}
}

// A cycle is created in problem with no ledger entry; leaving problem records
// that it was entered at creation, not at the moment it was left.
func TestApplyPhaseTransitionDatesProblemFromRecordCreation(t *testing.T) {
	record := NewCycleRecord("io-1", "/repo", "", "2026-10-24T01:00:00Z")
	result := ApplyPhaseTransition(record, model.IssueOpsPhaseGrill, "2026-10-24T01:05:00Z", "", "", nil)
	problem := result.PhaseLedger[model.IssueOpsPhaseProblem]
	if problem.EnteredAt != "2026-10-24T01:00:00Z" || problem.CompletedAt != "2026-10-24T01:05:00Z" {
		t.Fatalf("problem ledger=%+v", problem)
	}
	// An already dated entry is not rewritten.
	record.PhaseLedger = model.IssueOpsPhaseLedger{model.IssueOpsPhaseProblem: {EnteredAt: "2026-10-24T01:02:00Z"}}
	if got := ApplyPhaseTransition(record, model.IssueOpsPhaseGrill, "2026-10-24T01:05:00Z", "", "", nil).PhaseLedger[model.IssueOpsPhaseProblem].EnteredAt; got != "2026-10-24T01:02:00Z" {
		t.Fatalf("existing entered_at rewritten: %s", got)
	}
}
