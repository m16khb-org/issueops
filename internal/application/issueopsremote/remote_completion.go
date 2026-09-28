package issueopsremote

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type CompletionProvider interface {
	UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error)
	CloseIssue(port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error)
}
type CompletionProviderResolver func(string) (CompletionProvider, error)
type MergeVerifier func(model.IssueOpsRemoteArtifactVerification) error

type RemoteCompletionService struct {
	records   IssueRecordReader
	collector CompletionCollector
	receipts  CompletionReceipts
	resolve   CompletionProviderResolver
	verify    MergeVerifier
}

func NewRemoteCompletionService(records IssueRecordReader, collector CompletionCollector, receipts CompletionReceipts, resolve CompletionProviderResolver, verify MergeVerifier) *RemoteCompletionService {
	return &RemoteCompletionService{records: records, collector: collector, receipts: receipts, resolve: resolve, verify: verify}
}

func (s *RemoteCompletionService) prepare(ctx context.Context, id, providerOverride string) (CompletionProvider, error) {
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	providerName := firstNonEmpty(providerOverride, domain.ResolveRecordProvider(record))
	if providerName == "" {
		return nil, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	provider, err := s.resolve(providerName)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateCompletionMerge(record); err != nil {
		return nil, err
	}
	if s.verify == nil {
		return nil, fmt.Errorf("merge verification is not configured")
	}
	if err := s.verify(*record.RemoteArtifact); err != nil {
		return nil, fmt.Errorf("merge evidence readback failed (refusing to continue): %w", err)
	}
	if provider == nil {
		return nil, fmt.Errorf("no issue provider configured")
	}
	return provider, nil
}

func (s *RemoteCompletionService) Reflect(ctx context.Context, id, providerOverride string, confirm bool) (model.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	var result port.IssueProviderUpdateIssueBodySectionResult
	provider, err := s.prepare(ctx, id, providerOverride)
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return record, result, err
	}
	if err := domain.ValidateReflectCompletion(record); err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	completion := s.collector.Collect(record)
	result, err = provider.UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{Repo: record.Repo, IssueURL: record.IssueURL, Section: model.IssueBodySectionCompletion, Completion: &completion, Confirm: confirm})
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	if !confirm || !result.Updated {
		return record, result, nil
	}
	record, err = s.receipts.Reflected(ctx, id)
	return record, result, err
}

func (s *RemoteCompletionService) Close(ctx context.Context, id, providerOverride string, confirm bool) (model.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
	var result port.IssueProviderCloseIssueResult
	provider, err := s.prepare(ctx, id, providerOverride)
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return record, result, err
	}
	if err := domain.ValidateCloseIssue(record); err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	result, err = provider.CloseIssue(port.IssueProviderCloseIssueRequest{Repo: record.Repo, IssueURL: record.IssueURL, Confirm: confirm})
	if err != nil {
		return model.IssueOpsRecord{}, result, err
	}
	if !confirm || !result.Closed {
		return record, result, nil
	}
	record, err = s.receipts.Closed(ctx, id)
	return record, result, err
}
