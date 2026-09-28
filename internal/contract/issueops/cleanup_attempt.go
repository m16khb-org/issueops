package issueops

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type CleanupOperation string

const (
	CleanupOperationFinish       CleanupOperation = "finish"
	CleanupOperationRemoteBranch CleanupOperation = "remote-branch"
)

// IssueOpsCleanupAttempt binds one cleanup operation to its execution owner.
// It remains present until that execution has drained its child processes.
type IssueOpsCleanupAttempt struct {
	Operation CleanupOperation `json:"operation"`
	Token     string           `json:"token"`
	StartedAt string           `json:"started_at"`
}

func ValidateCleanupAttempt(attempt *IssueOpsCleanupAttempt) error {
	if attempt == nil {
		return nil
	}
	if attempt.Operation != CleanupOperationFinish && attempt.Operation != CleanupOperationRemoteBranch {
		return fmt.Errorf("issueops cleanup attempt operation is invalid")
	}
	if len(attempt.Token) != 64 || strings.ToLower(attempt.Token) != attempt.Token {
		return fmt.Errorf("issueops cleanup attempt token is invalid")
	}
	if _, err := hex.DecodeString(attempt.Token); err != nil {
		return fmt.Errorf("issueops cleanup attempt token is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, attempt.StartedAt); err != nil {
		return fmt.Errorf("issueops cleanup attempt timestamp is invalid")
	}
	return nil
}
