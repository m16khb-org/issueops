package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

// ArmCleanupAbandon arms an abandon attempt and stores the failure receipt on
// the record. It requires the abandon operation, delegates the arm to
// ArmCleanup, and releases a claimable execution lease at the attempt start
// time.
func ArmCleanupAbandon(record model.Record, attempt model.CleanupAttempt, failure model.CleanupAbandonFailure) (model.Record, error) {
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

// ApplyCleanupAbandonFailure stores failure as the abandon receipt and stamps
// the record with its time. It also clears the cleanup attempt when the apply
// drained.
func ApplyCleanupAbandonFailure(record model.Record, failure model.CleanupAbandonFailure, drained bool) model.Record {
	record.CleanupAbandonFailure = &failure
	record.UpdatedAt = failure.At
	if drained {
		record.CleanupAttempt = nil
	}
	return record
}
