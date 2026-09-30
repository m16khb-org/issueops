package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

func (s CleanupRecordStore) ArmAbandon(ctx context.Context, expected model.CleanupSnapshot, attempt model.IssueOpsCleanupAttempt, failure model.IssueOpsCleanupAbandonFailure) (model.CleanupSnapshot, error) {
	record, err := cleanupSnapshotRecord(expected, false)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	next, err := domain.ArmCleanupAbandon(record, attempt, failure)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	saved, err := s.save(ctx, expected, next)
	if _, ok := errors.AsType[port.RawCASFailure](err); ok {
		return saved, fmt.Errorf("abandon authority changed before local cleanup CAS: %w", err)
	}
	return saved, err
}
func (s CleanupRecordStore) FailAbandon(ctx context.Context, expected model.CleanupSnapshot, failure model.IssueOpsCleanupAbandonFailure, drained bool) (model.CleanupSnapshot, error) {
	record, err := cleanupOperationRecord(expected, model.CleanupOperationAbandon)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	return s.save(ctx, expected, domain.ApplyCleanupAbandonFailure(record, failure, drained))
}
func (s CleanupRecordStore) DeleteAbandoned(ctx context.Context, expected model.CleanupSnapshot) ([]string, error) {
	record, err := cleanupOperationRecord(expected, model.CleanupOperationAbandon)
	if err != nil {
		return nil, err
	}
	db, err := sqlstore.Open(s.StateRoot)
	if err != nil {
		return nil, err
	}
	var deleted []string
	err = db.WithSpan(ctx, func(spanCtx context.Context) error {
		return db.CompareAndApplyFunc(spanCtx, []port.ExpectedRecord{{Bucket: issueOpsBucket, ID: record.ID, Data: []byte(expected.Revision)}}, func() ([]port.RecordMutation, error) {
			mutations := []port.RecordMutation{}
			rows := []string{}
			for _, id := range domain.CleanupAbandonIntentOperationIDs(record) {
				data, found, err := db.Get(externalIntentBucket, id)
				if err != nil {
					return nil, err
				}
				if !found {
					continue
				}
				var owner struct {
					LifecycleID string `json:"lifecycle_id"`
				}
				if err := json.Unmarshal(data, &owner); err != nil {
					return nil, fmt.Errorf("decode external intent payload %s: %w", id, err)
				}
				if owner.LifecycleID != record.ID {
					return nil, fmt.Errorf("refusing to delete external intent row %s owned by another lifecycle", id)
				}
				mutations = append(mutations, port.RecordMutation{Bucket: externalIntentBucket, ID: id, Delete: true})
				rows = append(rows, id)
			}
			mutations = append(mutations, port.RecordMutation{Bucket: artifactStageBucket, ID: record.ID, Delete: true}, port.RecordMutation{Bucket: issueOpsBucket, ID: record.ID, Delete: true})
			deleted = rows
			return mutations, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

var _ port.CleanupAbandonRecords = CleanupRecordStore{}
