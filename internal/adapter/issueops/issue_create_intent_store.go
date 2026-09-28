package issueops

import (
	"context"

	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

type IssueCreateIntentStore struct{ StateRoot string }

func (s IssueCreateIntentStore) Update(ctx context.Context, id string, transition application.IssueIntentTransition) (model.IssueOpsRecord, error) {
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

var _ application.IssueIntentStore = IssueCreateIntentStore{}
