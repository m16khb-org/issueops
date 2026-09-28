package port

import (
	"context"

	model "issueops/internal/contract/issueops"
)

// CleanupOwnershipRecords requires an exclusive lifetime lease at the caller.
// Arm may recover only a crashed attempt of the same operation.
type CleanupOwnershipRecords interface {
	Load(context.Context, string) (model.CleanupSnapshot, error)
	Arm(context.Context, model.CleanupSnapshot, model.IssueOpsCleanupAttempt) (model.CleanupSnapshot, error)
	Check(context.Context, model.CleanupSnapshot) error
	MarkAuditReflected(context.Context, model.CleanupSnapshot, string) (model.CleanupSnapshot, error)
}

type CleanupFinishRecords interface {
	CleanupOwnershipRecords
	Fail(context.Context, model.CleanupSnapshot, model.IssueOpsCleanupFinishFailure, bool) (model.CleanupSnapshot, error)
	Delete(context.Context, model.CleanupSnapshot) error
}

type CleanupRemoteBranchRecords interface {
	CleanupOwnershipRecords
	// Release is allowed only after successful inherited-process drainage.
	Release(context.Context, model.CleanupSnapshot, string) (model.CleanupSnapshot, error)
}

// CleanupFinishObservationError distinguishes evidence failures from a planned
// cleanup result so transports preserve their existing error response contract.
type CleanupFinishObservationError struct{ Err error }

func (e *CleanupFinishObservationError) Error() string { return e.Err.Error() }
func (e *CleanupFinishObservationError) Unwrap() error { return e.Err }
