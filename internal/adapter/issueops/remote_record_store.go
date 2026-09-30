package issueops

import (
	"context"

	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

type RemoteRecordStore struct{ StateRoot string }

func (s RemoteRecordStore) WithinTransaction(ctx context.Context, id string, fn func(context.Context) error) error {
	return withIssueOpsLock(ctx, s.StateRoot, id, fn)
}

func (s RemoteRecordStore) Read(_ context.Context, id string) (model.IssueOpsRecord, error) {
	return ReadIssueOps(s.StateRoot, id)
}

func (s RemoteRecordStore) Update(ctx context.Context, id string, transition application.RecordTransition) (model.IssueOpsRecord, error) {
	var persisted model.IssueOpsRecord
	err := withIssueOpsLock(ctx, s.StateRoot, id, func(context.Context) error {
		current, err := ReadIssueOps(s.StateRoot, id)
		if err != nil {
			return err
		}
		updated, err := transition(current)
		if err != nil {
			return err
		}
		persisted, err = writeIssueOps(s.StateRoot, updated)
		return err
	})
	return persisted, err
}

var _ application.RecordStore = RemoteRecordStore{}
