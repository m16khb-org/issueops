package issueopsremote

import (
	"context"
	"fmt"
	"strings"

	reportcontract "issueops/internal/contract/artifactreadability"
	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/artifactreadability"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// CompletionProvider 타입은 완료 반영에 쓰는 provider 포트로, 이슈 본문 구역을 갱신하고 이슈를 닫는다.
type CompletionProvider interface {
	UpdateIssueBodySection(context.Context, port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error)
	CloseIssue(context.Context, port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error)
}
type completionProviderResolver func(string) (CompletionProvider, error)

// MergeVerifier 타입은 기록된 원격 아티팩트가 병합되었는지 확인하는 함수다.
type MergeVerifier func(model.RemoteArtifactVerification) error

// RemoteCompletionService 타입은 완료한 사이클의 결과를 이슈 본문에 반영하고 이슈를 닫은 뒤 영수증을 기록에 남긴다.
type RemoteCompletionService struct {
	records   issueRecordReader
	materials TrackedMaterials
	receipts  CompletionReceipts
	resolve   completionProviderResolver
	verify    MergeVerifier
}

// NewRemoteCompletionService 함수는 기록 조회, 추적 자료, 영수증, provider 해석 함수, 병합 검증기를 묶어 서비스를 만든다.
func NewRemoteCompletionService(records issueRecordReader, materials TrackedMaterials, receipts CompletionReceipts, resolve completionProviderResolver, verify MergeVerifier) *RemoteCompletionService {
	return &RemoteCompletionService{records: records, materials: materials, receipts: receipts, resolve: resolve, verify: verify}
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

// Reflect 메서드는 완료 보고 본문을 가독성 검사해 이슈 본문의 완료 구역에 반영한다. confirm이면 본문이 필요하고 가독성 위반은 거부하며, 성공하면 반영 영수증을 기록한다.
func (s *RemoteCompletionService) Reflect(ctx context.Context, id, providerOverride, resultBody string, confirm bool) (model.Record, port.IssueProviderUpdateIssueBodySectionResult, reportcontract.Report, error) {
	var result port.IssueProviderUpdateIssueBodySectionResult
	var report reportcontract.Report
	resultBody = strings.TrimSpace(resultBody)
	if confirm && resultBody == "" {
		return model.Record{}, result, report, fmt.Errorf("--body-file is required with --confirm: write the progress report for human readers first")
	}
	provider, err := s.prepare(ctx, id, providerOverride)
	if err != nil {
		return model.Record{}, result, report, err
	}
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return record, result, report, err
	}
	if err := domain.ValidateReflectCompletion(record); err != nil {
		return model.Record{}, result, report, err
	}
	report = artifactreadability.Check(artifactreadability.Input{Kind: artifactreadability.KindCompletion, Body: resultBody})
	if s.materials.missing(record) {
		report.Warnings = append(report.Warnings, reportcontract.Finding{Code: "tracked_materials_missing", Message: trackedMaterialsMissingWarning})
	}
	if confirm && !report.OK {
		return model.Record{}, result, report, artifactreadability.RefusalError(report)
	}
	completion := domain.ProjectRemoteCompletion(record)
	completion.ResultBody = resultBody
	result, err = provider.UpdateIssueBodySection(ctx, port.IssueProviderUpdateIssueBodySectionRequest{Repo: record.Repo, IssueURL: record.IssueURL, Section: model.IssueBodySectionCompletion, Completion: &completion, Confirm: confirm})
	if err != nil {
		return model.Record{}, result, report, idempotentRetryHint(err, confirm, "reflected")
	}
	if !confirm || !result.Updated {
		return record, result, report, nil
	}
	record, err = s.receipts.reflected(ctx, id)
	return record, result, report, err
}

// Close 메서드는 완료한 사이클의 이슈를 닫는다. 이미 닫은 영수증이 있으면 미리보기는 provider를 부르지 않고, confirm이 성공하면 종료 영수증을 기록한다.
func (s *RemoteCompletionService) Close(ctx context.Context, id, providerOverride string, confirm bool) (model.Record, port.IssueProviderCloseIssueResult, error) {
	var result port.IssueProviderCloseIssueResult
	provider, err := s.prepare(ctx, id, providerOverride)
	if err != nil {
		return model.Record{}, result, err
	}
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return record, result, err
	}
	if err := domain.ValidateCloseIssue(record); err != nil {
		return model.Record{}, result, err
	}
	// QA: a preview after the confirmed close still said closed:false and
	// "would execute gh issue close"; the record already holds the receipt.
	if !confirm && record.RemoteCompletion != nil && record.RemoteCompletion.IssueClosedAt != "" {
		return record, port.IssueProviderCloseIssueResult{OK: true, Provider: firstNonEmpty(providerOverride, domain.ResolveRecordProvider(record)), IssueURL: record.IssueURL, Closed: true, AlreadyClosed: true,
			Preview: "already closed at " + record.RemoteCompletion.IssueClosedAt + " (recorded receipt); nothing to run"}, nil
	}
	result, err = provider.CloseIssue(ctx, port.IssueProviderCloseIssueRequest{Repo: record.Repo, IssueURL: record.IssueURL, Confirm: confirm})
	if err != nil {
		return model.Record{}, result, idempotentRetryHint(err, confirm, "closed")
	}
	if !confirm || !result.Closed {
		return record, result, nil
	}
	record, err = s.receipts.closed(ctx, id)
	return record, result, err
}

// idempotentRetryHint names the way back after a confirmed write failed (QA,
// lost responses: the agent had to read the issue back to learn that the
// section was written and the issue closed). Previews keep their error.
func idempotentRetryHint(err error, confirm bool, state string) error {
	if !confirm {
		return err
	}
	return domain.IdempotentConfirmRetry(err, "an issue already "+state)
}
