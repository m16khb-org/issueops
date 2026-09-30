package issueopsapp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type completionProvider struct {
	port.IssueProvider
	updates, closes int
	update          func(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error)
}

func (p *completionProvider) UpdateIssueBodySection(ctx context.Context, req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	p.updates++
	return p.update(req)
}

func (p *completionProvider) CloseIssue(_ context.Context, req port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	p.closes++
	return port.IssueProviderCloseIssueResult{OK: true, Closed: true, IssueURL: req.IssueURL}, nil
}

func TestRemoteCompletionCompositionVerifiesMergeBeforeEffectsAndPreservesLatestRecord(t *testing.T) {
	root := t.TempDir()
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "61-remote-completion"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/61"
	record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/62"}
	if _, err := issueops.WriteIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	prov := &completionProvider{}
	applied := false
	prov.update = func(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
		if req.Completion == nil || req.Section != model.IssueBodySectionCompletion || req.Completion.RemoteArtifactURL != record.RemoteArtifact.URL || req.Completion.ResultBody != completionResultBody {
			t.Fatalf("completion=%+v", req)
		}
		if applied {
			latest, err := issueops.ReadIssueOps(root, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			latest.AISlopCleanVerification = []string{"concurrent update"}
			if _, err := issueops.WriteIssueOps(root, latest); err != nil {
				t.Fatal(err)
			}
		}
		return port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: applied, URL: req.IssueURL}, nil
	}
	mergeErr := errors.New("merge readback failed")
	now := time.Date(2026, 9, 28, 1, 0, 0, 0, time.UTC)
	service := newRemoteCompletionService(root, func(string) (port.IssueProvider, error) { return prov, nil }, func(model.IssueOpsRemoteArtifactVerification) error { return mergeErr }, func() time.Time { return now })
	if _, _, _, err := service.Reflect(context.Background(), record.ID, "", completionResultBody, true); !errors.Is(err, mergeErr) || prov.updates != 0 {
		t.Fatalf("merge refusal: calls=%d err=%v", prov.updates, err)
	}
	if _, _, err := service.Close(context.Background(), record.ID, "", true); !errors.Is(err, mergeErr) || prov.closes != 0 {
		t.Fatalf("merge refusal: calls=%d err=%v", prov.closes, err)
	}
	mergeErr = nil
	got, _, _, err := service.Reflect(context.Background(), record.ID, "", completionResultBody, true)
	if err != nil || got.RemoteCompletion != nil {
		t.Fatalf("unapplied reflection stamped: %+v %v", got.RemoteCompletion, err)
	}
	applied = true
	got, _, _, err = service.Reflect(context.Background(), record.ID, "", completionResultBody, true)
	if err != nil || got.RemoteCompletion == nil || got.RemoteCompletion.ReflectedAt == "" || strings.Join(got.AISlopCleanVerification, ",") != "concurrent update" {
		t.Fatalf("reflection overwrote latest state: %+v %v", got.RemoteCompletion, err)
	}
	got, _, err = service.Close(context.Background(), record.ID, "", true)
	if err != nil {
		t.Fatal(err)
	}
	first := got.RemoteCompletion.IssueClosedAt
	now = now.Add(time.Hour)
	got, _, err = service.Close(context.Background(), record.ID, "", true)
	if err != nil || first == "" || got.RemoteCompletion.IssueClosedAt != first {
		t.Fatalf("repeat close changed first timestamp: %+v %v", got.RemoteCompletion, err)
	}
}

const completionResultBody = "이슈 본문을 읽기 쉽게 바꾸고 실제 명령 테스트로 검증했습니다. 동시 수정 결과가 보존되는 것도 확인했습니다."
