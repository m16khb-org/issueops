package issueops

import (
	"context"

	application "issueops/internal/application/issueopsbodysync"
	model "issueops/internal/contract/issueops"
)

type BodySyncRepository struct{ StateRoot string }

func (r BodySyncRepository) Read(_ context.Context, id string) (model.IssueOpsRecord, error) {
	return ReadIssueOps(r.StateRoot, id)
}

func (r BodySyncRepository) Update(ctx context.Context, id string, transition application.RecordTransition) (model.IssueOpsRecord, error) {
	var persisted model.IssueOpsRecord
	err := withIssueOpsLock(ctx, r.StateRoot, id, func(context.Context) error {
		current, err := ReadIssueOps(r.StateRoot, id)
		if err != nil {
			return err
		}
		updated, err := transition(current)
		if err != nil {
			return err
		}
		persisted, err = writeIssueOps(r.StateRoot, updated)
		return err
	})
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return persisted, nil
}

type BodySyncAuthority struct{}

func (BodySyncAuthority) Authorize(_ context.Context, record model.IssueOpsRecord, actor model.IssueOpsActor) error {
	return validateExecutionMutation(record, &actor)
}

var _ application.Repository = BodySyncRepository{}
var _ application.Authority = BodySyncAuthority{}
