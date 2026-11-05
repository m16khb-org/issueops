package port

import (
	"context"

	model "issueops/internal/contract/issueops"
)

// cleanupOwnershipRecords requires an exclusive lifetime lease at the caller.
// Arm may recover only a crashed attempt of the same operation.
type cleanupOwnershipRecords interface {
	Load(context.Context, string) (model.CleanupSnapshot, error)
	Arm(context.Context, model.CleanupSnapshot, model.CleanupAttempt) (model.CleanupSnapshot, error)
	Check(context.Context, model.CleanupSnapshot) error
}

// CleanupFinishRecords is the record surface cleanup finish uses once ownership is armed. Fail
// records a finish failure on the snapshot, with the bool telling whether workspace processes were
// already drained, and Delete removes the record and stores the outcome in one transaction.
type CleanupFinishRecords interface {
	cleanupOwnershipRecords
	Fail(context.Context, model.CleanupSnapshot, model.CleanupFinishFailure, bool) (model.CleanupSnapshot, error)
	// Delete removes the record and stores outcome in the same transaction.
	Delete(context.Context, model.CleanupSnapshot, model.CycleOutcome) error
}

// CleanupRemoteBranchRecords is the record surface remote-branch cleanup uses once ownership is
// armed, adding Release to hand the record back after the inherited processes drained.
type CleanupRemoteBranchRecords interface {
	cleanupOwnershipRecords
	// Release is allowed only after successful inherited-process drainage.
	Release(context.Context, model.CleanupSnapshot, string) (model.CleanupSnapshot, error)
}

// CleanupFinishObservationError distinguishes evidence failures from a planned
// cleanup result so transports preserve their existing error response contract.
type CleanupFinishObservationError struct{ Err error }

func (e *CleanupFinishObservationError) Error() string { return e.Err.Error() }
func (e *CleanupFinishObservationError) Unwrap() error { return e.Err }
