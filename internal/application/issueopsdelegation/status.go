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

type ChildStatusRecords interface {
	branchapp.CycleRecords
	Scan() ([]model.IssueOpsRecord, error)
}

type StatusService struct {
	Records   ChildStatusRecords
	Authority cycleapp.MutationAuthority
	Now       func() time.Time
}

func (s StatusService) Status(ctx context.Context, parentID string, repair bool, actor *model.IssueOpsActor) (model.IssueOpsChildStatusResult, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return model.IssueOpsChildStatusResult{OK: false}, fmt.Errorf("parent_id is required")
	}
	var parent model.IssueOpsRecord
	err := s.Records.WithinLock(ctx, parentID, func() error {
		var err error
		parent, err = s.Records.Load(parentID)
		return err
	})
	if err != nil {
		return model.IssueOpsChildStatusResult{OK: false, ParentID: parentID}, err
	}
	records, err := s.Records.Scan()
	if err != nil {
		return model.IssueOpsChildStatusResult{OK: false, ParentID: parent.ID}, err
	}
	scanned := domain.SelectChildren(parent, records)
	result := domain.BuildChildStatus(parent, scanned)
	if !repair {
		return result, nil
	}
	var appended []string
	err = s.Records.WithinLock(ctx, parent.ID, func() error {
		current, err := s.Records.Load(parent.ID)
		if err != nil {
			return err
		}
		if err := s.Authority.Validate(current, actor); err != nil {
			return err
		}
		updated, added := domain.RepairChildIndex(current, scanned)
		appended = added
		if len(added) == 0 {
			return nil
		}
		updated.UpdatedAt = s.Now().UTC().Format(time.RFC3339Nano)
		_, err = s.Records.Save(updated)
		return err
	})
	if err != nil {
		return result, err
	}
	result.RepairAppended = appended
	result.Repaired = len(appended) > 0
	return result, nil
}
