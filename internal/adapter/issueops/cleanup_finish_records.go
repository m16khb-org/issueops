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

// FinishRecordStore bypasses ordinary-writer fences only through exact revision
// and attempt checks. The application must hold the finish lifetime lease.
type FinishRecordStore struct{ StateRoot string }

func (s FinishRecordStore) Load(ctx context.Context, id string) (model.CleanupFinishSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	if _, err := normalizeIssueOpsID(id); err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	raw, found, err := sqlstore.GetExisting(s.StateRoot, issueOpsBucket, id)
	if err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	if !found {
		return model.CleanupFinishSnapshot{}, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	}
	record, err := decodeIssueOpsRecord(id, raw)
	if err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	return model.CleanupFinishSnapshot{Record: record, Revision: string(raw)}, nil
}

func (s FinishRecordStore) Arm(ctx context.Context, expected model.CleanupFinishSnapshot, attempt model.IssueOpsCleanupFinishAttempt) (model.CleanupFinishSnapshot, error) {
	record, err := finishSnapshotRecord(expected, false)
	if err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	next, err := domain.ArmCleanupFinish(record, attempt)
	if err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	return s.save(ctx, expected, next)
}

func (s FinishRecordStore) Check(ctx context.Context, expected model.CleanupFinishSnapshot) error {
	if _, err := finishSnapshotRecord(expected, true); err != nil {
		return err
	}
	// CompareAndApply deliberately skips empty mutations. Its callback variant
	// performs the revision comparison even for this read-only ownership check.
	return s.compare(ctx, expected, nil)
}

func (s FinishRecordStore) Fail(ctx context.Context, expected model.CleanupFinishSnapshot, failure model.IssueOpsCleanupFinishFailure, drained bool) (model.CleanupFinishSnapshot, error) {
	record, err := finishSnapshotRecord(expected, true)
	if err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	return s.save(ctx, expected, domain.ApplyCleanupFinishFailure(record, failure, drained))
}

func (s FinishRecordStore) MarkAuditReflected(ctx context.Context, expected model.CleanupFinishSnapshot, now string) (model.CleanupFinishSnapshot, error) {
	record, err := finishSnapshotRecord(expected, true)
	if err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	return s.save(ctx, expected, domain.MarkRemoteCompletionReflected(record, now))
}

func (s FinishRecordStore) Delete(ctx context.Context, expected model.CleanupFinishSnapshot) error {
	if _, err := finishSnapshotRecord(expected, true); err != nil {
		return err
	}
	return s.compare(ctx, expected, []port.RecordMutation{
		{Bucket: issueOpsBucket, ID: expected.Record.ID, Delete: true},
		{Bucket: artifactStageBucket, ID: expected.Record.ID, Delete: true},
	})
}

func (s FinishRecordStore) save(ctx context.Context, expected model.CleanupFinishSnapshot, next model.IssueOpsRecord) (model.CleanupFinishSnapshot, error) {
	encoded, raw, err := encodeIssueOpsRecord(next)
	if err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	if err := s.compare(ctx, expected, []port.RecordMutation{{Bucket: issueOpsBucket, ID: encoded.ID, Data: raw}}); err != nil {
		return model.CleanupFinishSnapshot{}, err
	}
	return model.CleanupFinishSnapshot{Record: encoded, Revision: string(raw)}, nil
}

func (s FinishRecordStore) compare(ctx context.Context, expected model.CleanupFinishSnapshot, mutations []port.RecordMutation) error {
	db, err := sqlstore.Open(s.StateRoot)
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, func(spanCtx context.Context) error {
		return db.CompareAndApplyFunc(spanCtx, []port.ExpectedRecord{{Bucket: issueOpsBucket, ID: expected.Record.ID, Data: []byte(expected.Revision)}}, func() ([]port.RecordMutation, error) { return mutations, nil })
	})
}

func finishSnapshotRecord(snapshot model.CleanupFinishSnapshot, owned bool) (model.IssueOpsRecord, error) {
	record, err := decodeIssueOpsRecord(snapshot.Record.ID, []byte(snapshot.Revision))
	if err != nil {
		return model.IssueOpsRecord{}, err
	}
	if owned {
		token := ""
		if snapshot.Record.CleanupFinishAttempt != nil {
			token = snapshot.Record.CleanupFinishAttempt.Token
		}
		if err := domain.ValidateCleanupFinishOwner(record, token); err != nil {
			return model.IssueOpsRecord{}, err
		}
	}
	return record, nil
}

var _ port.CleanupFinishRecords = FinishRecordStore{}
