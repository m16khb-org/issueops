package issueops

import model "issueops/internal/contract/issueops"

func ApplyCleanupFinishFailure(record model.IssueOpsRecord, failure model.IssueOpsCleanupFinishFailure, drained bool) model.IssueOpsRecord {
	record.CleanupFinishFailure = &failure
	record.UpdatedAt = failure.At
	if drained {
		record.CleanupAttempt = nil
	}
	return record
}
