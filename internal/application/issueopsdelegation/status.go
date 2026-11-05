package issueopsdelegation

import (
	"context"
	"fmt"
	"strings"
	"time"

	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type childStatusRecords interface {
	branchapp.CycleRecords
	Scan() ([]model.Record, error)
}

// StatusService reports the delegated children of a parent cycle and can
// repair the parent's child index.
type StatusService struct {
	Records   childStatusRecords
	Authority cycleapp.MutationAuthority
	Now       func() time.Time
}

// Status returns the parent's children as derived from a records scan. With
// repair set it also appends children missing from the parent's index, after
// validating the actor's authority, and reports the appended IDs. An empty
// parentID is an error.
func (s StatusService) Status(ctx context.Context, parentID string, repair bool, actor *model.Actor) (model.ChildStatusResult, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return model.ChildStatusResult{OK: false}, fmt.Errorf("parent_id is required (--parent: the parent cycle's io-… id)")
	}
	var parent model.Record
	err := s.Records.WithinLock(ctx, parentID, func(spanCtx context.Context) error {
		var err error
		parent, err = s.Records.Load(parentID)
		return err
	})
	if err != nil {
		return model.ChildStatusResult{OK: false, ParentID: parentID}, err
	}
	records, err := s.Records.Scan()
	if err != nil {
		return model.ChildStatusResult{OK: false, ParentID: parent.ID}, err
	}
	scanned := domain.SelectChildren(parent, records)
	result := domain.BuildChildStatus(parent, scanned)
	if !repair {
		return result, nil
	}
	var appended []string
	err = s.Records.WithinLock(ctx, parent.ID, func(spanCtx context.Context) error {
		current, err := s.Records.Load(parent.ID)
		if err != nil {
			return err
		}
		if err := s.Authority.Validate(ctx, current, actor); err != nil {
			return err
		}
		updated, added := domain.RepairChildIndex(current, scanned)
		appended = added
		if len(added) == 0 {
			return nil
		}
		updated.UpdatedAt = s.Now().UTC().Format(time.RFC3339Nano)
		_, err = s.Records.Save(spanCtx, updated)
		return err
	})
	if err != nil {
		return result, err
	}
	result.RepairAppended = appended
	result.Repaired = len(appended) > 0
	return result, nil
}
