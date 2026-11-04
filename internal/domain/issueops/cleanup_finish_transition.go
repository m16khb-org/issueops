package issueops

import model "issueops/internal/contract/issueops"

// ApplyCleanupFinishFailure stores failure as the cleanup finish receipt and
// stamps the record with its time. It also clears the cleanup attempt when
// the apply drained.
func ApplyCleanupFinishFailure(record model.Record, failure model.CleanupFinishFailure, drained bool) model.Record {
	record.CleanupFinishFailure = &failure
	record.UpdatedAt = failure.At
	if drained {
		record.CleanupAttempt = nil
	}
	return record
}
