package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestFinishAttemptTransitionsPreserveOwnershipAndInput(t *testing.T) {
	attempt := model.IssueOpsCleanupAttempt{Operation: "finish", Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
	original := model.IssueOpsRecord{ID: "io-finish"}
	armed, err := ArmCleanup(original, attempt)
	if err != nil {
		t.Fatal(err)
	}
	if original.CleanupAttempt != nil || armed.CleanupAttempt == nil {
		t.Fatal("arm aliased its input")
	}
	if _, err := ArmCleanup(armed, attempt); err == nil {
		t.Fatal("reused attempt token")
	}
	if err := ValidateCleanupOwner(armed, model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationFinish, Token: "wrong", StartedAt: attempt.StartedAt}); err == nil {
		t.Fatal("accepted foreign owner")
	}
	failure := model.IssueOpsCleanupFinishFailure{Step: model.CleanupFailureStepWorktreeRemove, Message: "failure", At: "2026-09-29T00:00:01Z"}
	retained := ApplyCleanupFinishFailure(armed, failure, false)
	cleared := ApplyCleanupFinishFailure(retained, failure, true)
	if retained.CleanupAttempt == nil || cleared.CleanupAttempt != nil || armed.CleanupFinishFailure != nil {
		t.Fatal("failure mutated ownership or original")
	}
	abandoned := original
	abandoned.CleanupAbandonFailure = &model.IssueOpsCleanupAbandonFailure{Step: model.CleanupFailureStepApplying}
	if _, err := ArmCleanup(abandoned, attempt); err == nil {
		t.Fatal("armed during abandon")
	}
}
