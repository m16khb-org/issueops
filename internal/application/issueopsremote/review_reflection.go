package issueopsremote

import (
	"context"
	"fmt"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type ReviewReflectionStore interface {
	IssueRecordReader
	RecordStore
}
type ReviewReflectionProvider interface {
	UpdateIssueBodySection(context.Context, port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error)
}
type ReviewProviderResolver func(string) (ReviewReflectionProvider, error)
type AncestryObserver func() ([]model.NativeProcessReceipt, error)

type ReviewReflectionService struct {
	store     ReviewReflectionStore
	authority Authority
	resolve   ReviewProviderResolver
	observe   AncestryObserver
	now       func() time.Time
}

func NewReviewReflectionService(store ReviewReflectionStore, authority Authority, resolve ReviewProviderResolver, observe AncestryObserver, now func() time.Time) *ReviewReflectionService {
	return &ReviewReflectionService{store: store, authority: authority, resolve: resolve, observe: observe, now: now}
}

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
