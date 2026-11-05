package issueopsremote

import (
	"context"
	"fmt"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"time"
)

// ChildCreateIntents 타입은 자식 이슈 생성 작업의 시작·결과·완료를 기록에 durable하게 남긴다. 모든 갱신은 행위자 권한 검사를 거친다.
type ChildCreateIntents struct {
	Store     RecordStore
	Authority authority
	Now       func() time.Time
}

func (s *ChildCreateIntents) timestamp() string { return s.Now().UTC().Format(time.RFC3339Nano) }
func (s *ChildCreateIntents) update(ctx context.Context, id string, actor model.Actor, transition RecordTransition) (model.Record, error) {
	return s.Store.Update(ctx, id, func(record model.Record) (model.Record, error) {
		if err := s.Authority.Authorize(ctx, record, actor); err != nil {
			return record, err
		}
		return transition(record)
	})
}

// Begin 메서드는 봉인된 자식 생성 작업을 기록에 시작으로 남기고 선택된 작업과 provider를 실제로 호출해야 하는지를 돌려준다.
func (s *ChildCreateIntents) Begin(ctx context.Context, id string, op model.ChildCreateOperation, explicitID string, actor model.Actor) (model.ChildCreateOperation, bool, error) {
	var selected model.ChildCreateOperation
	var invoke bool
	_, err := s.update(ctx, id, actor, func(record model.Record) (model.Record, error) {
		updated, current, shouldInvoke, err := domain.BeginChildOperation(record, op, explicitID)
		selected, invoke = current, shouldInvoke
		return updated, err
	})
	return selected, invoke, err
}
func (s *ChildCreateIntents) outcome(ctx context.Context, id, operation, status, url, failure string, actor model.Actor) error {
	_, err := s.update(ctx, id, actor, func(record model.Record) (model.Record, error) {
		return domain.RecordChildOutcome(record, operation, status, url, failure, s.timestamp())
	})
	return err
}

// Complete 메서드는 관찰한 작업이 검증 중 바뀌지 않았는지 확인한 뒤 완료로 표시한다. reconcile이 true이면 작업을 현재 holder에 다시 묶는다.
func (s *ChildCreateIntents) Complete(ctx context.Context, expected model.Record, observed model.ChildCreateOperation, url string, actor model.Actor, reconcile bool) error {
	_, err := s.update(ctx, expected.ID, actor, func(record model.Record) (model.Record, error) {
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

// childCrashReleased is the failure a settled crash records: not_invoked lets
// the same create-child run again.
const childCrashReleased = "the create-child caller died before the provider created the child; settled: no candidate, no create running, quiet period passed"

// releaseCrashed records not_invoked for the exact pending attempt that was
// settled (compare-and-swap on status, attempt, and updated_at).
func (s *ChildCreateIntents) releaseCrashed(ctx context.Context, observed model.Record, op model.ChildCreateOperation, actor model.Actor) error {
	_, err := s.update(ctx, observed.ID, actor, func(record model.Record) (model.Record, error) {
		if !sameChildRecoveryAuthority(observed, record) {
			return record, fmt.Errorf("child recovery authority changed")
		}
		current, ok := domain.FindChildOperation(record, op.OperationID, "")
		if !ok || current.Status != model.IssueCreateIntentPending || current.Attempt != op.Attempt || current.UpdatedAt != op.UpdatedAt || current.CanonicalURL != "" {
			return record, fmt.Errorf("child operation changed since it was settled; run reconcile-child again")
		}
		// The dead caller's lease was often taken over: bind the operation to
		// the current holder first, like the other recovery outcomes.
		rebound, err := domain.RebindChildOperation(record, op.OperationID)
		if err != nil {
			return record, err
		}
		return domain.RecordChildOutcome(rebound, op.OperationID, model.IssueCreateIntentNotInvoked, "", childCrashReleased, s.timestamp())
	})
	return err
}

func (s *ChildCreateIntents) recoveryFailure(ctx context.Context, observed model.Record, op model.ChildCreateOperation, url, status, failure string, actor model.Actor) error {
	_, err := s.update(ctx, observed.ID, actor, func(record model.Record) (model.Record, error) {
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
