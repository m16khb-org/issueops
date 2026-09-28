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
