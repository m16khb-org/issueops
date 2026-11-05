package issueops

import (
	"fmt"
	"strings"
)

// BaseSyncRequiredError reports that a completed execution must sync with its
// base before it can proceed, and carries the preview command that starts the sync.
type BaseSyncRequiredError struct {
	ID                   string
	CompletionGeneration uint64
	NextCommand          string
}

// NewBaseSyncRequiredError returns the error for cycle id at completionGeneration,
// with NextCommand set to the shell-quoted sync-base preview command.
func NewBaseSyncRequiredError(id string, completionGeneration uint64) *BaseSyncRequiredError {
	quotedID := "'" + strings.ReplaceAll(id, "'", "'\\''") + "'"
	return &BaseSyncRequiredError{
		ID:                   id,
		CompletionGeneration: completionGeneration,
		NextCommand: fmt.Sprintf(
			"issueops execution sync-base --id %s --preview --completion-generation %d --json",
			quotedID,
			completionGeneration,
		),
	}
}

func (e *BaseSyncRequiredError) Error() string {
	return fmt.Sprintf("post-completion sync-base is required for %s at completion generation %d", e.ID, e.CompletionGeneration)
}

// IssueOpsErrorFields returns the structured fields (code, completion generation,
// next command) that the CLI error envelope adds to the message.
func (e *BaseSyncRequiredError) IssueOpsErrorFields() map[string]any {
	return map[string]any{
		"code":                  "post_completion_sync_base_required",
		"completion_generation": e.CompletionGeneration,
		"next_command":          e.NextCommand,
	}
}
