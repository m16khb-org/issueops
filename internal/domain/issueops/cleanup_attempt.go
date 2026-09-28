package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

// RequireNoCleanupAttempt fences ordinary mutations, including crash recovery.
// Only the matching cleanup executor may replace or clear a persisted attempt.
func RequireNoCleanupAttempt(attempt *model.IssueOpsCleanupAttempt) error {
	if attempt != nil {
		return fmt.Errorf("cleanup %s apply is in progress", attempt.Operation)
	}
	return nil
}

// ValidateCleanupOperationAccess allows recovery only through the operation
// that owns the abandoned attempt. The caller must first acquire its lifetime.
func ValidateCleanupOperationAccess(record model.IssueOpsRecord, operation model.CleanupOperation) error {
	if attempt := record.CleanupAttempt; attempt != nil && attempt.Operation != operation {
		return fmt.Errorf("cleanup %s owns this cycle; resume cleanup %s before cleanup %s", attempt.Operation, attempt.Operation, operation)
	}
	return nil
}

func ArmCleanup(record model.IssueOpsRecord, attempt model.IssueOpsCleanupAttempt) (model.IssueOpsRecord, error) {
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

func ValidateCleanupOwner(record model.IssueOpsRecord, expected model.IssueOpsCleanupAttempt) error {
	if expected.Token == "" || record.CleanupAttempt == nil || *record.CleanupAttempt != expected {
		return fmt.Errorf("cleanup attempt ownership changed")
	}
	return nil
}

func ReleaseCleanupAttempt(record model.IssueOpsRecord, now string) model.IssueOpsRecord {
	record.CleanupAttempt = nil
	record.UpdatedAt = now
	return record
}
