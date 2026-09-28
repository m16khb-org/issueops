package port

import (
	"context"

	model "issueops/internal/contract/issueops"
)

// CleanupFinishRecords is reserved for the exclusive finish executor. Arm may
// replace a crashed attempt only while that executor holds the lifetime lease.
// Fail(clear=true) and Delete require successful process drainage by the caller.
type CleanupFinishRecords interface {
	Load(context.Context, string) (model.CleanupFinishSnapshot, error)
	Arm(context.Context, model.CleanupFinishSnapshot, model.IssueOpsCleanupFinishAttempt) (model.CleanupFinishSnapshot, error)
	Check(context.Context, model.CleanupFinishSnapshot) error
	Fail(context.Context, model.CleanupFinishSnapshot, model.IssueOpsCleanupFinishFailure, bool) (model.CleanupFinishSnapshot, error)
	MarkAuditReflected(context.Context, model.CleanupFinishSnapshot, string) (model.CleanupFinishSnapshot, error)
	Delete(context.Context, model.CleanupFinishSnapshot) error
}

// CleanupFinishObservationError distinguishes evidence failures from a planned
// cleanup result so transports preserve their existing error response contract.
type CleanupFinishObservationError struct{ Err error }

func (e *CleanupFinishObservationError) Error() string { return e.Err.Error() }
func (e *CleanupFinishObservationError) Unwrap() error { return e.Err }
