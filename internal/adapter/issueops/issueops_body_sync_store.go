package issueops

import (
	"context"

	application "issueops/internal/application/issueopsbodysync"
	model "issueops/internal/contract/issueops"
)

// BodySyncRepository reads and updates cycle records for artifact body sync.
type BodySyncRepository struct{ StateRoot string }

func (r BodySyncRepository) Read(_ context.Context, id string) (model.Record, error) {
	return ReadIssueOps(r.StateRoot, id)
}

// Update applies transition to the current record of id under the cycle lock and persists
// the result. On failure it returns a record with OK false.
func (r BodySyncRepository) Update(ctx context.Context, id string, transition application.RecordTransition) (model.Record, error) {
	var persisted model.Record
	err := withIssueOpsLock(ctx, r.StateRoot, id, func(spanCtx context.Context) error {
		current, err := ReadIssueOps(r.StateRoot, id)
		if err != nil {
			return err
		}
		updated, err := transition(current)
		if err != nil {
			return err
		}
		persisted, err = writeIssueOps(spanCtx, r.StateRoot, updated)
		return err
	})
	if err != nil {
		return model.Record{OK: false}, err
	}
	return persisted, nil
}

var _ application.Repository = BodySyncRepository{}
