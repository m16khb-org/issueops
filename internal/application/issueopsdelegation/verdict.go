package issueopsdelegation

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"time"

	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

// Validator records the parent's verdict (accepted, rejected, dropped) on a
// delegated child cycle.
type Validator struct {
	Records   branchapp.CycleRecords
	Authority cycleapp.MutationAuthority
	Now       func() time.Time
}

// Accept marks the child as accepted by the parent, attaching evidence.
func (s Validator) Accept(ctx context.Context, parentID, childID string, evidence []string, actor *model.Actor) (model.ChildValidationResult, error) {
	return s.validate(ctx, parentID, childID, "accepted", "", evidence, actor)
}

// Reject marks the child as rejected by the parent with a reason and evidence.
func (s Validator) Reject(ctx context.Context, parentID, childID, reason string, evidence []string, actor *model.Actor) (model.ChildValidationResult, error) {
	return s.validate(ctx, parentID, childID, "rejected", reason, evidence, actor)
}

// Drop marks the child as dropped by the parent with a reason, without evidence.
func (s Validator) Drop(ctx context.Context, parentID, childID, reason string, actor *model.Actor) (model.ChildValidationResult, error) {
	return s.validate(ctx, parentID, childID, "dropped", reason, nil, actor)
}
func (s Validator) validate(ctx context.Context, parentID, childID, verdict, reason string, evidence []string, actor *model.Actor) (model.ChildValidationResult, error) {
	parentID, childID = strings.TrimSpace(parentID), strings.TrimSpace(childID)
	result := model.ChildValidationResult{OK: false, ParentID: parentID, ChildID: childID}
	reason, evidence, err := domain.PrepareChildVerdict(verdict, reason, evidence)
	if err != nil {
		return result, err
	}
	if err := domain.ValidateChildValidationIDs(parentID, childID); err != nil {
		return result, err
	}
	var child model.Record
	err = s.Records.WithinLock(ctx, childID, func(spanCtx context.Context) error {
		var err error
		child, err = s.Records.Load(childID)
		return err
	})
	archived := errors.Is(err, fs.ErrNotExist) && verdict != "rejected"
	if err != nil && !archived {
		return result, err
	}
	if !archived {
		if err := domain.ValidateChildForVerdict(child, parentID, verdict); err != nil {
			return result, err
		}
	}
	err = s.Records.WithinLock(ctx, parentID, func(spanCtx context.Context) error {
		ref, err := s.applyVerdictToParent(ctx, spanCtx, parentID, childID, verdict, reason, evidence, actor, archived)
		if err != nil {
			return err
		}
		result.ParentRef = ref
		return nil
	})
	if err != nil {
		return result, err
	}
	result.OK = true
	return result, nil
}

// applyVerdictToParent는 부모 잠금 안에서 권한을 확인하고 자식 판정을 부모 레코드에 반영해 저장한다.
func (s Validator) applyVerdictToParent(ctx, spanCtx context.Context, parentID, childID, verdict, reason string, evidence []string, actor *model.Actor, archived bool) (model.ChildCycleRef, error) {
	var ref model.ChildCycleRef
	parent, err := s.Records.Load(parentID)
	if err != nil {
		return ref, err
	}
	if err := s.Authority.Validate(ctx, parent, actor); err != nil {
		return ref, err
	}
	current, readErr := s.Records.Load(childID)
	now := s.Now().UTC().Format(time.RFC3339Nano)
	if archived {
		if readErr == nil {
			return ref, domain.ValidateArchivedChildObservation(current, parentID)
		}
		if !errors.Is(readErr, fs.ErrNotExist) {
			return ref, readErr
		}
		parent, ref, err = domain.ApplyArchivedChildVerdict(parent, childID, verdict, reason, evidence, now)
	} else {
		if readErr != nil {
			return ref, readErr
		}
		if err := domain.ValidateChildMutation(current); err != nil {
			return ref, err
		}
		if err := domain.ValidateChildForVerdict(current, parentID, verdict); err != nil {
			return ref, err
		}
		parent, ref, err = domain.ApplyChildVerdict(parent, current, verdict, reason, evidence, now)
	}
	if err != nil {
		return ref, err
	}
	parent.UpdatedAt = now
	if _, err := s.Records.Save(spanCtx, parent); err != nil {
		return ref, err
	}
	return ref, nil
}
