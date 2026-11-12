// Package issueopsretention holds the domain rule that decides which issueops
// cycle records are old and finished enough to prune.
package issueopsretention

import (
	"time"

	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
	issueopsinventorydomain "issueops/internal/domain/issueopsinventory"
)

// IsPrunable reports whether record may be deleted by retention: the cycle is
// done with no cleanup attempt, any lease is released, any issue creation and
// remote artifact completion is finished, its UpdatedAt is a valid non-zero
// instant, and its last record instant is before cutoff. The last record
// instant is the latest stored timestamp, the last_record_at that `issueops
// list` shows. It is never earlier than UpdatedAt, so this keeps every record
// an UpdatedAt-only age would keep.
func IsPrunable(record issueopsretentioncontract.Record, cutoff time.Time) bool {
	if record.CleanupAttempt != nil {
		return false
	}
	if record.Phase != issueopsretentioncontract.PhaseDone {
		return false
	}
	if record.Execution != nil && record.Execution.Lease.Status != issueopsretentioncontract.LeaseStatusReleased {
		return false
	}
	if record.IssueCreateIntent != nil &&
		record.IssueCreateIntent.Status != issueopsretentioncontract.IssueCreateCompleted {
		return false
	}
	if record.RemoteArtifact != nil &&
		(record.RemoteCompletion == nil || record.RemoteCompletion.ReflectedAt == "") {
		return false
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, record.UpdatedAt)
	if err != nil || updatedAt.IsZero() {
		return false
	}
	lastRecordAt, err := time.Parse(time.RFC3339Nano, issueopsinventorydomain.LastRecordAt(record))
	return err == nil && lastRecordAt.Before(cutoff)
}
