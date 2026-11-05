package issueops

import (
	"context"

	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

// RemoteRecordStore reads and updates cycle records for remote operations.
type RemoteRecordStore struct{ StateRoot string }

// WithinTransaction runs fn while holding the per-cycle lock for id.
func (s RemoteRecordStore) WithinTransaction(ctx context.Context, id string, fn func(context.Context) error) error {
	return withIssueOpsLock(ctx, s.StateRoot, id, fn)
}

func (s RemoteRecordStore) Read(_ context.Context, id string) (model.Record, error) {
	return ReadIssueOps(s.StateRoot, id)
}

// Update applies transition to the current record of id under the cycle lock and returns
// the persisted record.
func (s RemoteRecordStore) Update(ctx context.Context, id string, transition application.RecordTransition) (model.Record, error) {
	var persisted model.Record
	err := withIssueOpsLock(ctx, s.StateRoot, id, func(spanCtx context.Context) error {
		current, err := ReadIssueOps(s.StateRoot, id)
		if err != nil {
			return err
		}
		updated, err := transition(current)
		if err != nil {
			return err
		}
		persisted, err = writeIssueOps(spanCtx, s.StateRoot, updated)
		return err
	})
	return persisted, err
}

var _ application.RecordStore = RemoteRecordStore{}
