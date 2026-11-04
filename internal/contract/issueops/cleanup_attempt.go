package issueops

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// CleanupOperation names which destructive cleanup command owns a
// CleanupAttempt.
type CleanupOperation string

// The CleanupOperation values a CleanupAttempt may carry. CleanupOperationFinish
// is post-merge cleanup, CleanupOperationAbandon is local teardown of an
// unmerged cycle, and CleanupOperationRemoteBranch deletes the remote branch.
const (
	CleanupOperationFinish       CleanupOperation = "finish"
	CleanupOperationAbandon      CleanupOperation = "abandon"
	CleanupOperationRemoteBranch CleanupOperation = "remote-branch"
)

// CleanupAttempt binds one cleanup operation to its execution owner.
// It remains present until that execution has drained its child processes.
type CleanupAttempt struct {
	Operation CleanupOperation `json:"operation"`
	Token     string           `json:"token"`
	StartedAt string           `json:"started_at"`
}

// ValidateCleanupAttempt reports whether attempt is a well-formed sealed
// attempt: a known Operation, a 64-character lowercase hex Token, and an RFC 3339
// StartedAt. A nil attempt is valid because no cleanup is in flight.
func ValidateCleanupAttempt(attempt *CleanupAttempt) error {
	if attempt == nil {
		return nil
	}
	if attempt.Operation != CleanupOperationFinish && attempt.Operation != CleanupOperationRemoteBranch && attempt.Operation != CleanupOperationAbandon {
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
