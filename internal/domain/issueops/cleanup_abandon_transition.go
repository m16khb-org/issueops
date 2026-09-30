package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

func ArmCleanupAbandon(record model.IssueOpsRecord, attempt model.IssueOpsCleanupAttempt, failure model.IssueOpsCleanupAbandonFailure) (model.IssueOpsRecord, error) {
	if attempt.Operation != model.CleanupOperationAbandon {
		return record, fmt.Errorf("abandon requires its own cleanup operation")
	}
	next, err := ArmCleanup(record, attempt)
	if err != nil {
		return record, err
	}
	next.CleanupAbandonFailure = &failure
	if next.Execution != nil && next.Execution.Lease.Status == model.LeaseStatusClaimable {
		execution := *next.Execution
		execution.Lease.Status = model.LeaseStatusReleased
		execution.Lease.ClaimTokenSHA256 = ""
		execution.Lease.ReleasedAt = attempt.StartedAt
		next.Execution = &execution
	}
	return next, nil
}

func ApplyCleanupAbandonFailure(record model.IssueOpsRecord, failure model.IssueOpsCleanupAbandonFailure, drained bool) model.IssueOpsRecord {
	record.CleanupAbandonFailure = &failure
	record.UpdatedAt = failure.At
	if drained {
		record.CleanupAttempt = nil
	}
	return record
}
