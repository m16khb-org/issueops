package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestFinishAttemptTransitionsPreserveOwnershipAndInput(t *testing.T) {
	attempt := model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
	original := model.IssueOpsRecord{ID: "io-finish"}
	armed, err := ArmCleanupFinish(original, attempt)
	if err != nil {
		t.Fatal(err)
	}
	if original.CleanupFinishAttempt != nil || armed.CleanupFinishAttempt == nil {
		t.Fatal("arm aliased its input")
	}
	if _, err := ArmCleanupFinish(armed, attempt); err == nil {
		t.Fatal("reused attempt token")
	}
	if err := ValidateCleanupFinishOwner(armed, "wrong"); err == nil {
		t.Fatal("accepted foreign owner")
	}
	failure := model.IssueOpsCleanupFinishFailure{Step: model.CleanupFailureStepWorktreeRemove, Message: "failure", At: "2026-09-29T00:00:01Z"}
	retained := ApplyCleanupFinishFailure(armed, failure, false)
	cleared := ApplyCleanupFinishFailure(retained, failure, true)
	if retained.CleanupFinishAttempt == nil || cleared.CleanupFinishAttempt != nil || armed.CleanupFinishFailure != nil {
		t.Fatal("failure mutated ownership or original")
	}
	abandoned := original
	abandoned.CleanupAbandonFailure = &model.IssueOpsCleanupAbandonFailure{Step: model.CleanupFailureStepApplying}
	if _, err := ArmCleanupFinish(abandoned, attempt); err == nil {
		t.Fatal("armed during abandon")
	}
}
