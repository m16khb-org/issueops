package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

// RequireNoCleanupAttempt fences ordinary mutations, including crash recovery.
// Only the matching cleanup executor may replace or clear a persisted attempt.
func RequireNoCleanupAttempt(attempt *model.CleanupAttempt) error {
	if attempt != nil {
		return fmt.Errorf("cleanup %s apply is in progress", attempt.Operation)
	}
	return nil
}

// ValidateCleanupOperationAccess allows recovery only through the operation
// that owns the abandoned attempt. The caller must first acquire its lifetime.
func ValidateCleanupOperationAccess(record model.Record, operation model.CleanupOperation) error {
	if attempt := record.CleanupAttempt; attempt != nil && attempt.Operation != operation {
		return fmt.Errorf("cleanup %s owns this cycle; resume cleanup %s before cleanup %s", attempt.Operation, attempt.Operation, operation)
	}
	return nil
}

// ArmCleanup records attempt as the active cleanup attempt of record. It
// validates the attempt, requires that no other cleanup operation owns the
// cycle, blocks non-abandon operations while an abandon apply is in progress,
// and rejects a reused attempt token.
func ArmCleanup(record model.Record, attempt model.CleanupAttempt) (model.Record, error) {
	if err := model.ValidateCleanupAttempt(&attempt); err != nil {
		return record, err
	}
	if err := ValidateCleanupOperationAccess(record, attempt.Operation); err != nil {
		return record, err
	}
	if attempt.Operation != model.CleanupOperationAbandon && record.CleanupAbandonFailure != nil && record.CleanupAbandonFailure.Step == model.CleanupFailureStepApplying {
		return record, fmt.Errorf("cleanup abandon apply is in progress")
	}
	if record.CleanupAttempt != nil && record.CleanupAttempt.Token == attempt.Token {
		return record, fmt.Errorf("cleanup %s requires a new attempt token", attempt.Operation)
	}
	record.CleanupAttempt = &attempt
	record.UpdatedAt = attempt.StartedAt
	return record, nil
}

// ValidateCleanupOwner fails unless the record still holds exactly the
// expected cleanup attempt with a non-empty token, so a stale executor cannot
// continue.
func ValidateCleanupOwner(record model.Record, expected model.CleanupAttempt) error {
	if expected.Token == "" || record.CleanupAttempt == nil || *record.CleanupAttempt != expected {
		return fmt.Errorf("cleanup attempt ownership changed")
	}
	return nil
}

// ReleaseCleanupAttempt clears the cleanup attempt of record and stamps now
// as the update time.
func ReleaseCleanupAttempt(record model.Record, now string) model.Record {
	record.CleanupAttempt = nil
	record.UpdatedAt = now
	return record
}
