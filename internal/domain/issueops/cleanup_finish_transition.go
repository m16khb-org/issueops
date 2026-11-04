package issueops

import model "issueops/internal/contract/issueops"

func ApplyCleanupFinishFailure(record model.Record, failure model.CleanupFinishFailure, drained bool) model.Record {
	record.CleanupFinishFailure = &failure
	record.UpdatedAt = failure.At
	if drained {
		record.CleanupAttempt = nil
	}
	return record
}
