package issueopsremote

import (
	"context"
	"fmt"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type reviewReflectionStore interface {
	issueRecordReader
	RecordStore
}

// ReviewReflectionProvider 타입은 리뷰 결과 반영에 쓰는 provider 포트로, 이슈 본문 구역을 갱신한다.
type ReviewReflectionProvider interface {
	UpdateIssueBodySection(context.Context, port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error)
}
type reviewProviderResolver func(string) (ReviewReflectionProvider, error)

// AncestryObserver 타입은 현재 프로세스의 조상 프로세스 영수증을 관찰해 행위자 증명에 쓰는 함수다.
type AncestryObserver func() ([]model.NativeProcessReceipt, error)

// ReviewReflectionService 타입은 devil's advocate 리뷰 결과를 이슈 본문 구역에 반영하고 반영 영수증을 기록에 남긴다.
type ReviewReflectionService struct {
	store     reviewReflectionStore
	authority authority
	resolve   reviewProviderResolver
	observe   AncestryObserver
	now       func() time.Time
}

// NewReviewReflectionService 함수는 저장소, 권한 검사, provider 해석 함수, 조상 관찰기, 시계를 묶어 서비스를 만든다.
func NewReviewReflectionService(store reviewReflectionStore, authority authority, resolve reviewProviderResolver, observe AncestryObserver, now func() time.Time) *ReviewReflectionService {
	return &ReviewReflectionService{store: store, authority: authority, resolve: resolve, observe: observe, now: now}
}

// Reflect 메서드는 기록의 리뷰 결과를 provider 이슈 본문에 반영한다. 조상 프로세스를 관찰한 행위자로 권한을 확인하고, confirm이 성공하면 반영 영수증을 남긴다.
func (s *ReviewReflectionService) Reflect(ctx context.Context, id, providerOverride string, confirm bool, actor model.Actor) (model.Record, port.IssueProviderUpdateIssueBodySectionResult, error) {
	var result port.IssueProviderUpdateIssueBodySectionResult
	record, err := s.store.Read(ctx, id)
	if err != nil {
		return model.Record{}, result, err
	}
	name := firstNonEmpty(providerOverride, domain.ResolveRecordProvider(record))
	if name == "" {
		return model.Record{}, result, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	provider, err := s.resolve(name)
	if err != nil {
		return model.Record{}, result, err
	}
	actor.NativeProcessAncestry, err = s.observe()
	if err != nil {
		return model.Record{}, result, err
	}
	if provider == nil {
		return model.Record{}, result, fmt.Errorf("no issue provider configured")
	}
	record, err = s.store.Read(ctx, id)
	if err != nil {
		return record, result, err
	}
	if err := s.authority.Authorize(ctx, record, actor); err != nil {
		return model.Record{}, result, err
	}
	if err := domain.ValidateReviewReflection(record); err != nil {
		return model.Record{}, result, err
	}
	result, err = provider.UpdateIssueBodySection(ctx, port.IssueProviderUpdateIssueBodySectionRequest{Repo: record.Repo, IssueURL: record.IssueURL, Section: model.IssueBodySectionDevilsAdvocate, Findings: record.DevilsAdvocateReview.Findings, Verdict: record.DevilsAdvocateReview.Verdict, Rounds: domain.PlanReviewRounds(record.DevilsAdvocateReview, len(record.RegressEvents)), ReviewerContext: record.DevilsAdvocateReview.ReviewerContext, Confirm: confirm})
	if err != nil {
		return model.Record{}, result, idempotentRetryHint(err, confirm, "reflected")
	}
	if !confirm || !result.Updated {
		return record, result, nil
	}
	return s.markReflected(ctx, id, actor, result)
}

// markReflected stamps the reflection receipt after a confirmed remote update.
func (s *ReviewReflectionService) markReflected(ctx context.Context, id string, actor model.Actor, result port.IssueProviderUpdateIssueBodySectionResult) (model.Record, port.IssueProviderUpdateIssueBodySectionResult, error) {
	// A confirmed remote update must retain the existing uncancelled receipt write.
	persist := context.WithoutCancel(ctx)
	record, err := s.store.Update(persist, id, func(current model.Record) (model.Record, error) {
		if err := domain.ValidateReviewReflectionStamp(current); err != nil {
			return current, err
		}
		if err := s.authority.Authorize(persist, current, actor); err != nil {
			return current, err
		}
		return domain.MarkReviewReflected(current, s.now().UTC().Format(time.RFC3339Nano)), nil
	})
	if err != nil {
		return model.Record{}, result, err
	}
	return record, result, nil
}
