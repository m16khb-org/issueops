package issueops

import (
	"context"
	"fmt"
	"io/fs"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// CleanupRecordStore bypasses ordinary-writer fences only through exact revision
// and attempt checks. The application must hold the cleanup lifetime lease.
type CleanupRecordStore struct{ StateRoot string }

func (s CleanupRecordStore) Load(ctx context.Context, id string) (model.CleanupSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return model.CleanupSnapshot{}, err
	}
	if _, err := normalizeIssueOpsID(id); err != nil {
		return model.CleanupSnapshot{}, err
	}
	raw, found, err := sqlstore.GetExisting(s.StateRoot, issueOpsBucket, id)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	if !found {
		return model.CleanupSnapshot{}, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	}
	record, err := decodeIssueOpsRecord(id, raw)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	return model.CleanupSnapshot{Record: record, Revision: string(raw)}, nil
}

func (s CleanupRecordStore) Arm(ctx context.Context, expected model.CleanupSnapshot, attempt model.IssueOpsCleanupAttempt) (model.CleanupSnapshot, error) {
	if attempt.Operation == model.CleanupOperationAbandon {
		return model.CleanupSnapshot{}, fmt.Errorf("abandon requires its sealed inventory arm")
	}
	record, err := cleanupSnapshotRecord(expected, false)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	next, err := domain.ArmCleanup(record, attempt)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	return s.save(ctx, expected, next)
}

func (s CleanupRecordStore) Check(ctx context.Context, expected model.CleanupSnapshot) error {
	if _, err := cleanupSnapshotRecord(expected, true); err != nil {
		return err
	}
	// CompareAndApply deliberately skips empty mutations. Its callback variant
	// performs the revision comparison even for this read-only ownership check.
	return s.compare(ctx, expected, nil)
}

func (s CleanupRecordStore) Fail(ctx context.Context, expected model.CleanupSnapshot, failure model.IssueOpsCleanupFinishFailure, drained bool) (model.CleanupSnapshot, error) {
	record, err := cleanupOperationRecord(expected, model.CleanupOperationFinish)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	return s.save(ctx, expected, domain.ApplyCleanupFinishFailure(record, failure, drained))
}

func (s CleanupRecordStore) Delete(ctx context.Context, expected model.CleanupSnapshot) error {
	if _, err := cleanupOperationRecord(expected, model.CleanupOperationFinish); err != nil {
		return err
	}
	return s.compare(ctx, expected, []port.RecordMutation{
		{Bucket: issueOpsBucket, ID: expected.Record.ID, Delete: true},
		{Bucket: artifactStageBucket, ID: expected.Record.ID, Delete: true},
	})
}

func (s CleanupRecordStore) save(ctx context.Context, expected model.CleanupSnapshot, next model.IssueOpsRecord) (model.CleanupSnapshot, error) {
	encoded, raw, err := encodeIssueOpsRecord(next)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	if err := s.compare(ctx, expected, []port.RecordMutation{{Bucket: issueOpsBucket, ID: encoded.ID, Data: raw}}); err != nil {
		return model.CleanupSnapshot{}, err
	}
	return model.CleanupSnapshot{Record: encoded, Revision: string(raw)}, nil
}

func (s CleanupRecordStore) compare(ctx context.Context, expected model.CleanupSnapshot, mutations []port.RecordMutation) error {
	db, err := sqlstore.Open(s.StateRoot)
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, func(spanCtx context.Context) error {
		return db.CompareAndApplyFunc(spanCtx, []port.ExpectedRecord{{Bucket: issueOpsBucket, ID: expected.Record.ID, Data: []byte(expected.Revision)}}, func() ([]port.RecordMutation, error) { return mutations, nil })
	})
}

func cleanupSnapshotRecord(snapshot model.CleanupSnapshot, owned bool) (model.IssueOpsRecord, error) {
	record, err := decodeIssueOpsRecord(snapshot.Record.ID, []byte(snapshot.Revision))
	if err != nil {
		return model.IssueOpsRecord{}, err
	}
	if owned {
		expected := model.IssueOpsCleanupAttempt{}
		if snapshot.Record.CleanupAttempt != nil {
			expected = *snapshot.Record.CleanupAttempt
		}
		if err := domain.ValidateCleanupOwner(record, expected); err != nil {
			return model.IssueOpsRecord{}, err
		}
	}
	return record, nil
}

var _ port.CleanupFinishRecords = CleanupRecordStore{}

func cleanupOperationRecord(snapshot model.CleanupSnapshot, operation model.CleanupOperation) (model.IssueOpsRecord, error) {
	record, err := cleanupSnapshotRecord(snapshot, true)
	if err != nil {
		return model.IssueOpsRecord{}, err
	}
	if err := domain.ValidateCleanupOperationAccess(record, operation); err != nil {
		return model.IssueOpsRecord{}, err
	}
	return record, nil
}

func (s CleanupRecordStore) Release(ctx context.Context, expected model.CleanupSnapshot, now string) (model.CleanupSnapshot, error) {
	record, err := cleanupOperationRecord(expected, model.CleanupOperationRemoteBranch)
	if err != nil {
		return model.CleanupSnapshot{}, err
	}
	return s.save(ctx, expected, domain.ReleaseCleanupAttempt(record, now))
}

var _ port.CleanupRemoteBranchRecords = CleanupRecordStore{}
