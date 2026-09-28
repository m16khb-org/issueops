package issueops

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// IssueOpsCleanupFinishAttempt is the durable identity of one finish execution.
// It remains present until that execution has drained its child processes.
type IssueOpsCleanupFinishAttempt struct {
	Token     string `json:"token"`
	StartedAt string `json:"started_at"`
}

func ValidateCleanupFinishAttempt(attempt *IssueOpsCleanupFinishAttempt) error {
	if attempt == nil {
		return nil
	}
	if len(attempt.Token) != 64 || strings.ToLower(attempt.Token) != attempt.Token {
		return fmt.Errorf("issueops cleanup finish attempt token is invalid")
	}
	if _, err := hex.DecodeString(attempt.Token); err != nil {
		return fmt.Errorf("issueops cleanup finish attempt token is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, attempt.StartedAt); err != nil {
		return fmt.Errorf("issueops cleanup finish attempt timestamp is invalid")
	}
	return nil
}
