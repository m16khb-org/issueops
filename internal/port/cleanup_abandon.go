package port

import (
	"context"

	model "issueops/internal/contract/issueops"
)

// CleanupAbandonRecords binds all writes to the exact snapshot and attempt.
// Its caller holds the shared execution lifetime through the external effects.
type CleanupAbandonRecords interface {
	Load(context.Context, string) (model.CleanupSnapshot, error)
	ArmAbandon(context.Context, model.CleanupSnapshot, model.CleanupAttempt, model.CleanupAbandonFailure) (model.CleanupSnapshot, error)
	Check(context.Context, model.CleanupSnapshot) error
	FailAbandon(context.Context, model.CleanupSnapshot, model.CleanupAbandonFailure, bool) (model.CleanupSnapshot, error)
	// DeleteAbandoned removes the record and stores outcome in the same transaction.
	DeleteAbandoned(context.Context, model.CleanupSnapshot, model.CycleOutcome) ([]string, error)
}
