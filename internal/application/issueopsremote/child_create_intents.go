package issueopsremote

import (
	"context"
	"fmt"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"time"
)

type ChildCreateIntents struct {
	Store     RecordStore
	Authority PublicationAuthority
	Now       func() time.Time
}

func (s *ChildCreateIntents) timestamp() string { return s.Now().UTC().Format(time.RFC3339Nano) }
func (s *ChildCreateIntents) update(ctx context.Context, id string, actor model.IssueOpsActor, transition RecordTransition) (model.IssueOpsRecord, error) {
	return s.Store.Update(ctx, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		if err := s.Authority.Authorize(ctx, record, actor); err != nil {
			return record, err
		}
		return transition(record)
	})
}
func (s *ChildCreateIntents) Begin(ctx context.Context, id string, op model.ChildCreateOperation, explicitID string, actor model.IssueOpsActor) (model.ChildCreateOperation, bool, error) {
	var selected model.ChildCreateOperation
	var invoke bool
	_, err := s.update(ctx, id, actor, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		updated, current, shouldInvoke, err := domain.BeginChildOperation(record, op, explicitID)
		selected, invoke = current, shouldInvoke
		return updated, err
	})
	return selected, invoke, err
}
func (s *ChildCreateIntents) Outcome(ctx context.Context, id, operation, status, url, failure string, actor model.IssueOpsActor) error {
	_, err := s.update(ctx, id, actor, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		return domain.RecordChildOutcome(record, operation, status, url, failure, s.timestamp())
	})
	return err
}
func (s *ChildCreateIntents) Complete(ctx context.Context, expected model.IssueOpsRecord, observed model.ChildCreateOperation, url string, actor model.IssueOpsActor, reconcile bool) error {
	_, err := s.update(ctx, expected.ID, actor, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		if !sameChildRecoveryAuthority(expected, record) {
			return record, fmt.Errorf("child recovery authority changed")
		}
		current, ok := domain.FindChildOperation(record, observed.OperationID, "")
		if !ok || current.RequestSHA256 != observed.RequestSHA256 || current.ParentURL != observed.ParentURL || current.Status != observed.Status || current.CanonicalURL != observed.CanonicalURL {
			return record, fmt.Errorf("child operation changed during verification")
		}
		if reconcile {
			var err error
			record, err = domain.RebindChildOperation(record, current.OperationID)
			if err != nil {
				return record, err
			}
		}
		if current.Status == model.IssueCreateIntentCompleted {
			return record, nil
		}
		return domain.CompleteChildOperation(record, current.OperationID, url, s.timestamp())
	})
	return err
}

func (s *ChildCreateIntents) RecoveryFailure(ctx context.Context, observed model.IssueOpsRecord, op model.ChildCreateOperation, url, status, failure string, actor model.IssueOpsActor) error {
	_, err := s.update(ctx, observed.ID, actor, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		if !sameChildRecoveryAuthority(observed, record) {
			return record, fmt.Errorf("child recovery authority changed")
		}
		current, ok := domain.FindChildOperation(record, op.OperationID, "")
		if !ok || current.Status != op.Status || current.CanonicalURL != op.CanonicalURL || current.RequestSHA256 != op.RequestSHA256 {
			return record, fmt.Errorf("child operation changed during recovery")
		}
		if current.Status == model.IssueCreateIntentCompleted {
			return record, nil
		}
		updated, err := domain.RebindChildOperation(record, op.OperationID)
		if err != nil {
			return record, err
		}
		return domain.RecordChildOutcome(updated, op.OperationID, status, url, failure, s.timestamp())
	})
	return err
}
