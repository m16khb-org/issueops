package issueopsremote

import (
	"context"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

// CompletionReceipts 타입은 원격 완료(병합, 이슈 종료, 반영) 영수증을 기록에 남긴다.
type CompletionReceipts struct {
	store RecordStore
	now   func() time.Time
}

// NewCompletionReceipts 함수는 저장소와 시계로 CompletionReceipts를 만든다.
func NewCompletionReceipts(store RecordStore, now func() time.Time) CompletionReceipts {
	return CompletionReceipts{store: store, now: now}
}

func (s CompletionReceipts) reflected(ctx context.Context, id string) (model.Record, error) {
	return s.update(ctx, id, domain.MarkRemoteCompletionReflected)
}

func (s CompletionReceipts) closed(ctx context.Context, id string) (model.Record, error) {
	return s.update(ctx, id, domain.MarkRemoteIssueClosed)
}

// Merged 메서드는 원격 병합 영수증을 기록하고 갱신된 기록을 돌려준다.
func (s CompletionReceipts) Merged(ctx context.Context, id string) (model.Record, error) {
	return s.update(ctx, id, domain.MarkRemoteMerged)
}

func (s CompletionReceipts) update(ctx context.Context, id string, apply func(model.Record, string) model.Record) (model.Record, error) {
	record, err := s.store.Update(ctx, id, func(record model.Record) (model.Record, error) {
		return apply(record, s.now().UTC().Format(time.RFC3339Nano)), nil
	})
	if err != nil {
		return model.Record{OK: false}, err
	}
	return record, nil
}
