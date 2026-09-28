package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

// RequireNoFinishAttempt fences ordinary mutations, including crash recovery.
// Only the finish executor may replace or clear a persisted attempt.
func RequireNoFinishAttempt(attempt *model.IssueOpsCleanupFinishAttempt) error {
	if attempt != nil {
		return fmt.Errorf("cleanup finish apply is in progress")
	}
	return nil
}
