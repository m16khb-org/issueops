package issueopslease

import (
	"bytes"
	"context"
	"fmt"

	recordcodec "issueops/internal/adapter/outbound/issueopsrecord"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type ReseedRepository struct{ store port.TransactionalRecordStore }

func NewReseedRepository(store port.TransactionalRecordStore) *ReseedRepository {
	return &ReseedRepository{store: store}
}

func (r *ReseedRepository) LoadSnapshot(_ context.Context, id string) (leaseapp.ReseedSnapshot, error) {
	if r == nil || r.store == nil {
		return leaseapp.ReseedSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	data, ok, err := r.store.Get(recordBucket, id)
	if err != nil {
		return leaseapp.ReseedSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return leaseapp.ReseedSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", id))
	}
	record, err := decodeMutableLeaseRecord(id, data)
	if err != nil {
		return leaseapp.ReseedSnapshot{}, err
	}
	if record.Execution == nil {
		return leaseapp.ReseedSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	return leaseapp.ReseedSnapshot{Record: toApplicationRecord(record), Raw: append([]byte(nil), data...)}, nil
}

func (r *ReseedRepository) CommitReseed(ctx context.Context, snapshot leaseapp.ReseedSnapshot, next leaseapp.Record) (leaseapp.RepositoryResult, error) {
	if r == nil || r.store == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	if err := issueopsdomain.RequireNoCleanupAttempt(next.Stable.CleanupAttempt); err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	var result leaseapp.RepositoryResult
	var operationErr error
	spanErr := r.store.WithSpan(ctx, func(spanCtx context.Context) error {
		result, operationErr = r.commitReseedWithinSpan(spanCtx, snapshot, next)
		return operationErr
	})
	if operationErr != nil {
		return leaseapp.RepositoryResult{}, operationErr
	}
	if spanErr != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, spanErr)
	}
	return result, nil
}

func (r *ReseedRepository) commitReseedWithinSpan(spanCtx context.Context, snapshot leaseapp.ReseedSnapshot, next leaseapp.Record) (leaseapp.RepositoryResult, error) {
	data, ok, err := r.store.Get(recordBucket, snapshot.Record.ID)
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", snapshot.Record.ID))
	}
	if !bytes.Equal(data, snapshot.Raw) {
		return leaseapp.RepositoryResult{}, fmt.Errorf("stale raw record snapshot")
	}
	current, err := decodeMutableLeaseRecord(snapshot.Record.ID, data)
	if err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	if current.Execution == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	if current.Execution.Lease.Generation != snapshot.Record.Lease.Generation {
		return leaseapp.RepositoryResult{}, fmt.Errorf("stale lease generation: current=%d expected=%d", current.Execution.Lease.Generation, snapshot.Record.Lease.Generation)
	}
	encoded, err := recordcodec.EncodeLease(next.Stable)
	if err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if err := r.store.Apply(spanCtx, []port.RecordMutation{{Bucket: recordBucket, ID: next.ID, Data: encoded}}); err != nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	return leaseapp.RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
}
