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

// ReseedRepository 타입은 reseed에 필요한 lease 레코드의 스냅샷 읽기와 비교 후 쓰기를 트랜잭션 저장소 위에서 수행한다.
type ReseedRepository struct{ store port.TransactionalRecordStore }

// NewReseedRepository 함수는 store를 쓰는 ReseedRepository를 만든다.
func NewReseedRepository(store port.TransactionalRecordStore) *ReseedRepository {
	return &ReseedRepository{store: store}
}

// LoadSnapshot 메서드는 id의 레코드를 읽어 디코딩한 값과 원본 바이트 복사본을 함께 반환한다.
// 레코드가 없거나 execution이 준비되지 않았으면 persistence 오류를 반환한다.
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

// CommitReseed 메서드는 한 span 안에서 저장된 레코드가 snapshot의 원본 바이트 및 lease 세대와 같을 때만 next를 저장한다.
// 그 사이 레코드가 바뀌었으면 stale 오류를 반환한다.
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
