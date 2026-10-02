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

type Validator struct {
	Records   branchapp.CycleRecords
	Authority cycleapp.MutationAuthority
	Now       func() time.Time
}

func (s Validator) Accept(ctx context.Context, parentID, childID string, evidence []string, actor *model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return s.validate(ctx, parentID, childID, "accepted", "", evidence, actor)
}
func (s Validator) Reject(ctx context.Context, parentID, childID, reason string, evidence []string, actor *model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return s.validate(ctx, parentID, childID, "rejected", reason, evidence, actor)
}
func (s Validator) Drop(ctx context.Context, parentID, childID, reason string, actor *model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return s.validate(ctx, parentID, childID, "dropped", reason, nil, actor)
}
func (s Validator) validate(ctx context.Context, parentID, childID, verdict, reason string, evidence []string, actor *model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	parentID, childID = strings.TrimSpace(parentID), strings.TrimSpace(childID)
	result := model.IssueOpsChildValidationResult{OK: false, ParentID: parentID, ChildID: childID}
	reason, evidence, err := domain.PrepareChildVerdict(verdict, reason, evidence)
	if err != nil {
		return result, err
	}
	if err := domain.ValidateChildValidationIDs(parentID, childID); err != nil {
		return result, err
	}
	var child model.IssueOpsRecord
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
		parent, err := s.Records.Load(parentID)
		if err != nil {
			return err
		}
		if err := s.Authority.Validate(ctx, parent, actor); err != nil {
			return err
		}
		current, readErr := s.Records.Load(childID)
		now := s.Now().UTC().Format(time.RFC3339Nano)
		var ref model.IssueOpsChildCycleRef
		if archived {
			if readErr == nil {
				return domain.ValidateArchivedChildObservation(current, parentID)
			}
			if !errors.Is(readErr, fs.ErrNotExist) {
				return readErr
			}
			parent, ref, err = domain.ApplyArchivedChildVerdict(parent, childID, verdict, reason, evidence, now)
		} else {
			if readErr != nil {
				return readErr
			}
			if err := domain.ValidateChildMutation(current); err != nil {
				return err
			}
			if err := domain.ValidateChildForVerdict(current, parentID, verdict); err != nil {
				return err
			}
			parent, ref, err = domain.ApplyChildVerdict(parent, current, verdict, reason, evidence, now)
		}
		if err != nil {
			return err
		}
		parent.UpdatedAt = now
		if _, err := s.Records.Save(spanCtx, parent); err != nil {
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
