package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

func ArmCleanupFinish(record model.IssueOpsRecord, attempt model.IssueOpsCleanupFinishAttempt) (model.IssueOpsRecord, error) {
	if err := model.ValidateCleanupFinishAttempt(&attempt); err != nil {
		return record, err
	}
	if record.CleanupAbandonFailure != nil && record.CleanupAbandonFailure.Step == model.CleanupFailureStepApplying {
		return record, fmt.Errorf("cleanup abandon apply is in progress")
	}
	if record.CleanupFinishAttempt != nil && record.CleanupFinishAttempt.Token == attempt.Token {
		return record, fmt.Errorf("cleanup finish requires a new attempt token")
	}
	record.CleanupFinishAttempt = &attempt
	record.UpdatedAt = attempt.StartedAt
	return record, nil
}

func ValidateCleanupFinishOwner(record model.IssueOpsRecord, token string) error {
	if token == "" || record.CleanupFinishAttempt == nil || record.CleanupFinishAttempt.Token != token {
		return fmt.Errorf("cleanup finish attempt ownership changed")
	}
	return nil
}

func ApplyCleanupFinishFailure(record model.IssueOpsRecord, failure model.IssueOpsCleanupFinishFailure, drained bool) model.IssueOpsRecord {
	record.CleanupFinishFailure = &failure
	record.UpdatedAt = failure.At
	if drained {
		record.CleanupFinishAttempt = nil
	}
	return record
}
