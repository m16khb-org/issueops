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
type ReviewReflectionAuthority interface {
	Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error
}
type ReviewReflectionProvider interface {
	UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error)
}
type ReviewProviderResolver func(string) (ReviewReflectionProvider, error)
type AncestryObserver func() ([]model.NativeProcessReceipt, error)

type ReviewReflectionService struct {
	store     ReviewReflectionStore
	authority ReviewReflectionAuthority
	resolve   ReviewProviderResolver
	observe   AncestryObserver
	now       func() time.Time
}

func NewReviewReflectionService(store ReviewReflectionStore, authority ReviewReflectionAuthority, resolve ReviewProviderResolver, observe AncestryObserver, now func() time.Time) *ReviewReflectionService {
	return &ReviewReflectionService{store: store, authority: authority, resolve: resolve, observe: observe, now: now}
}

func (s *ReviewReflectionService) Reflect(ctx context.Context, id, providerOverride string, confirm bool, actor model.IssueOpsActor) (model.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	var result port.IssueProviderUpdateIssueBodySectionResult
	record, err := s.store.Read(ctx, id)
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	name := firstNonEmpty(providerOverride, domain.ResolveRecordProvider(record))
	if name == "" {
		return model.IssueOpsRecord{}, result, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	provider, err := s.resolve(name)
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	actor.NativeProcessAncestry, err = s.observe()
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	if provider == nil {
		return model.IssueOpsRecord{}, result, fmt.Errorf("no issue provider configured")
	}
	record, err = s.store.Read(ctx, id)
	if err != nil {
		return record, result, err
	}
	if err := s.authority.Authorize(ctx, record, actor); err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	if err := domain.ValidateReviewReflection(record); err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	result, err = provider.UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{Repo: record.Repo, IssueURL: record.IssueURL, Section: model.IssueBodySectionDevilsAdvocate, Findings: record.DevilsAdvocateReview.Findings, Confirm: confirm})
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	if !confirm || !result.Updated {
		return record, result, nil
	}
	// A confirmed remote update must retain the existing uncancelled receipt write.
	record, err = s.store.Update(context.Background(), id, func(current model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		if err := domain.ValidateReviewReflectionStamp(current); err != nil {
			return current, err
		}
		if err := s.authority.Authorize(ctx, current, actor); err != nil {
			return current, err
		}
		return domain.MarkReviewReflected(current, s.now().UTC().Format(time.RFC3339Nano)), nil
	})
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	return record, result, nil
}
