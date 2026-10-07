package issueopsbodysync

import (
	"context"
	"strings"
	"testing"
	"time"

	"issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
	bodysync "issueops/internal/domain/issueopsbodysync"
	"issueops/internal/port"
)

const (
	syncIssueURL = "https://github.com/o/r/issues/1"
	syncLiveBody = "## 요약\n\n예전 본문입니다."
	syncNewBody  = `## 요약

본문 동기화가 원격 본문을 새 계약으로 바꿉니다. 끝나면 팀원이 요약만 읽고 변경 이유를 압니다.

## 배경

원격 본문이 사이클의 결정과 달라져 팀원이 잘못된 범위를 읽습니다.

## 완료 기준

- 동기화한 본문의 첫 절이 요약입니다.

## 범위

- 하는 것: 본문 교체
- 하지 않는 것: 관리 구간 수정

## 검증

서비스 테스트로 교체 결과를 확인합니다.`
)

type fakeRepository struct {
	record  issueops.IssueOpsRecord
	updates int
}

func (r *fakeRepository) Read(context.Context, string) (issueops.IssueOpsRecord, error) {
	return r.record, nil
}

func (r *fakeRepository) Update(_ context.Context, _ string, transition RecordTransition) (issueops.IssueOpsRecord, error) {
	r.updates++
	next, err := transition(r.record)
	if err == nil {
		r.record = next
	}
	return next, err
}

type allowAll struct{}

func (allowAll) Authorize(context.Context, issueops.IssueOpsRecord, issueops.IssueOpsActor) error {
	return nil
}

type fakeProvider struct {
	body           string
	childVerified  bool
	reads, writes  int
	hierarchyCalls int
}

func (p *fakeProvider) Name() string { return "github" }

func (p *fakeProvider) ReadArtifactBody(_ context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	p.reads++
	return port.IssueProviderArtifactBody{Provider: "github", Kind: req.Kind, URL: req.URL, Body: p.body}, nil
}

func (p *fakeProvider) ReplaceArtifactBody(_ context.Context, req port.IssueProviderReplaceArtifactBodyRequest) (port.IssueProviderReplaceArtifactBodyResult, error) {
	if !req.Confirm {
		return port.IssueProviderReplaceArtifactBodyResult{OK: true, Preview: "dry-run"}, nil
	}
	p.writes++
	p.body = req.Body
	return port.IssueProviderReplaceArtifactBodyResult{OK: true, Updated: true, URL: req.URL, VerifiedBodySHA256: bodysync.SHA256Body(req.Body)}, nil
}

func (p *fakeProvider) VerifyChildHierarchy(context.Context, port.IssueProviderChildHierarchyRequest) (port.IssueProviderChildHierarchyResult, error) {
	p.hierarchyCalls++
	return port.IssueProviderChildHierarchyResult{Provider: "github", OK: true, Verified: p.childVerified}, nil
}

func newSyncService(provider *fakeProvider) (*Service, *fakeRepository) {
	repository := &fakeRepository{record: issueops.IssueOpsRecord{
		OK: true, ID: "io-sync", IssueURL: syncIssueURL,
		BodySyncs: []issueops.IssueOpsRemoteBodySync{{Kind: "issue", URL: syncIssueURL, ToSHA256: bodysync.SHA256Body(syncLiveBody), SyncedAt: "2026-10-01T00:00:00Z"}},
	}}
	now := func() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC) }
	return NewService(repository, provider, allowAll{}, now), repository
}

func TestSyncConfirmRefusesAStaleExpectedDigestWithoutWriting(t *testing.T) {
	provider := &fakeProvider{body: syncLiveBody}
	service, repository := newSyncService(provider)
	_, _, err := service.Sync(t.Context(), "io-sync", bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, ProposedBody: syncNewBody, Confirm: true,
		ExpectedBodySHA256: strings.Repeat("0", 64),
	}, issueops.IssueOpsActor{})
	if err == nil || !strings.Contains(err.Error(), "remote body changed since it was read") {
		t.Fatalf("stale digest error = %v", err)
	}
	if provider.writes != 0 || repository.updates != 0 {
		t.Fatalf("a refused sync must not write: writes=%d updates=%d", provider.writes, repository.updates)
	}
}

func TestSyncConfirmWritesAndRecordsTheVerifiedBaseline(t *testing.T) {
	provider := &fakeProvider{body: syncLiveBody}
	service, repository := newSyncService(provider)
	record, result, err := service.Sync(t.Context(), "io-sync", bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, ProposedBody: syncNewBody, Confirm: true,
		ExpectedBodySHA256: bodysync.SHA256Body(syncLiveBody),
	}, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated || provider.writes != 1 || repository.updates != 1 {
		t.Fatalf("updated=%v writes=%d updates=%d", result.Updated, provider.writes, repository.updates)
	}
	last := record.BodySyncs[len(record.BodySyncs)-1]
	if last.ToSHA256 != bodysync.SHA256Body(provider.body) || last.FromSHA256 != bodysync.SHA256Body(syncLiveBody) {
		t.Fatalf("baseline = %+v", last)
	}
}

func TestSyncChildRefusesAnUnverifiedHierarchyBeforeReadingTheBody(t *testing.T) {
	provider := &fakeProvider{body: syncLiveBody, childVerified: false}
	service, _ := newSyncService(provider)
	_, _, err := service.Sync(t.Context(), "io-sync", bodysynccontract.Command{
		Kind: bodysynccontract.KindIssue, URL: "https://github.com/o/r/issues/99", ProposedBody: syncNewBody,
	}, issueops.IssueOpsActor{})
	if err == nil || !strings.Contains(err.Error(), "not a provider-native child") {
		t.Fatalf("child error = %v", err)
	}
	if provider.hierarchyCalls != 1 || provider.reads != 0 {
		t.Fatalf("hierarchy=%d reads=%d, want the hierarchy check before any body read", provider.hierarchyCalls, provider.reads)
	}
}
