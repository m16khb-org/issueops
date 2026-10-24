package issueopsremote

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type completionProviderFake struct {
	update      port.IssueProviderUpdateIssueBodySectionResult
	updateErr   error
	closeResult port.IssueProviderCloseIssueResult
	closeErr    error
	updates     []port.IssueProviderUpdateIssueBodySectionRequest
	closes      []port.IssueProviderCloseIssueRequest
}

func (p *completionProviderFake) UpdateIssueBodySection(_ context.Context, req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	p.updates = append(p.updates, req)
	return p.update, p.updateErr
}

func (p *completionProviderFake) CloseIssue(_ context.Context, req port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	p.closes = append(p.closes, req)
	return p.closeResult, p.closeErr
}

const completionReport = "검증을 마치고 병합까지 끝냈습니다. 남은 작업은 없습니다."

func completionRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID: "io-done", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/4",
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/8"},
	}
}

type completionHarness struct {
	store    *artifactStoreForTest
	provider *completionProviderFake
	verified []model.IssueOpsRemoteArtifactVerification
	service  *RemoteCompletionService
}

func newCompletionHarness(record model.IssueOpsRecord, provider *completionProviderFake) *completionHarness {
	store, _ := newArtifactStoreForTest(record)
	h := &completionHarness{store: store, provider: provider}
	h.service = NewRemoteCompletionService(store, TrackedMaterials{}, NewCompletionReceipts(store, fixedNow),
		func(string) (CompletionProvider, error) { return provider, nil },
		func(a model.IssueOpsRemoteArtifactVerification) error { h.verified = append(h.verified, a); return nil })
	return h
}

func TestRemoteCompletionReflectConfirmStampsReceiptAfterVerifiedMerge(t *testing.T) {
	h := newCompletionHarness(completionRecord(), &completionProviderFake{update: port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: true}})
	record, result, report, err := h.service.Reflect(context.Background(), "io-done", "", "  "+completionReport+"\n", true)
	if err != nil || !result.Updated || !report.OK {
		t.Fatalf("result=%+v report=%+v err=%v", result, report, err)
	}
	if len(h.verified) != 1 || h.verified[0].URL != "https://github.com/acme/repo/pull/8" {
		t.Fatalf("merge evidence must be read back first: %+v", h.verified)
	}
	req := h.provider.updates[0]
	if req.Section != model.IssueBodySectionCompletion || !req.Confirm || req.Completion == nil || req.Completion.ResultBody != completionReport || req.Completion.RemoteArtifactURL != "https://github.com/acme/repo/pull/8" || req.IssueURL != "https://github.com/acme/repo/issues/4" {
		t.Fatalf("request = %+v", req)
	}
	if record.RemoteCompletion == nil || record.RemoteCompletion.ReflectedAt != "2026-03-04T05:06:07Z" || h.store.records["io-done"].RemoteCompletion == nil {
		t.Fatalf("receipt = %+v", record.RemoteCompletion)
	}
}

func TestRemoteCompletionReflectPreviewAndUnchangedDoNotStamp(t *testing.T) {
	for _, tc := range []struct {
		name    string
		confirm bool
		update  bool
	}{{"preview", false, true}, {"already reflected", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCompletionHarness(completionRecord(), &completionProviderFake{update: port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: tc.update}})
			record, _, _, err := h.service.Reflect(context.Background(), "io-done", "", completionReport, tc.confirm)
			if err != nil || record.RemoteCompletion != nil || h.store.records["io-done"].RemoteCompletion != nil {
				t.Fatalf("record=%+v err=%v", record, err)
			}
			if h.provider.updates[0].Confirm != tc.confirm {
				t.Fatalf("confirm flag must be forwarded: %+v", h.provider.updates[0])
			}
		})
	}
}

func TestRemoteCompletionReflectRefusesBeforeRemoteWrite(t *testing.T) {
	long := strings.Repeat("가", 2100)
	noIssue := completionRecord()
	noIssue.IssueURL = ""
	noIssue.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github"}
	for _, tc := range []struct {
		name    string
		record  model.IssueOpsRecord
		body    string
		confirm bool
		want    string
	}{
		{"confirm needs body", completionRecord(), " \n", true, "--body-file is required with --confirm"},
		{"no linked issue", noIssue, completionReport, true, "cannot reflect completion before a linked issue"},
		{"unreadable report on confirm", completionRecord(), long, true, "readability check failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCompletionHarness(tc.record, &completionProviderFake{})
			_, _, _, err := h.service.Reflect(context.Background(), "io-done", "", tc.body, tc.confirm)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(h.provider.updates) != 0 {
				t.Fatalf("err=%v updates=%d", err, len(h.provider.updates))
			}
		})
	}
}

func TestRemoteCompletionReflectPreviewReportsFindingsWithoutRefusing(t *testing.T) {
	h := newCompletionHarness(completionRecord(), &completionProviderFake{update: port.IssueProviderUpdateIssueBodySectionResult{OK: true}})
	_, _, report, err := h.service.Reflect(context.Background(), "io-done", "", strings.Repeat("가", 2100), false)
	if err != nil || report.OK || len(report.Critical) == 0 || len(h.provider.updates) != 1 {
		t.Fatalf("a preview shows the findings but still previews: report=%+v err=%v", report, err)
	}
}

func TestRemoteCompletionPrepareRefusals(t *testing.T) {
	boom := errors.New("boom")
	noProvider := completionRecord()
	noProvider.IssueURL = "https://example.com/issues/1"
	noProvider.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{URL: "u"}
	noArtifact := completionRecord()
	noArtifact.RemoteArtifact = nil
	for _, tc := range []struct {
		name     string
		id       string
		record   model.IssueOpsRecord
		resolve  CompletionProviderResolver
		verify   MergeVerifier
		noVerify bool
		override string
		want     string
	}{
		{name: "missing record", id: "io-missing", record: completionRecord(), want: "file does not exist"},
		{name: "unknown provider", id: "io-done", record: noProvider, want: "cannot determine provider"},
		{name: "resolver fails", id: "io-done", record: completionRecord(), resolve: func(string) (CompletionProvider, error) { return nil, boom }, want: "boom"},
		{name: "no verified artifact", id: "io-done", record: noArtifact, want: "before a verified remote artifact"},
		{name: "verifier not configured", id: "io-done", record: completionRecord(), noVerify: true, want: "merge verification is not configured"},
		{name: "readback fails", id: "io-done", record: completionRecord(), verify: func(model.IssueOpsRemoteArtifactVerification) error { return boom }, want: "merge evidence readback failed"},
		{name: "nil provider", id: "io-done", record: completionRecord(), resolve: func(string) (CompletionProvider, error) { return nil, nil }, want: "no issue provider configured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newArtifactStoreForTest(tc.record)
			provider := &completionProviderFake{}
			resolve := tc.resolve
			if resolve == nil {
				resolve = func(string) (CompletionProvider, error) { return provider, nil }
			}
			verify := tc.verify
			if verify == nil && !tc.noVerify {
				verify = func(model.IssueOpsRemoteArtifactVerification) error { return nil }
			}
			service := NewRemoteCompletionService(store, TrackedMaterials{}, NewCompletionReceipts(store, fixedNow), resolve, verify)
			_, _, err := service.Close(context.Background(), tc.id, tc.override, true)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(provider.closes) != 0 {
				t.Fatalf("err=%v closes=%d, want %q", err, len(provider.closes), tc.want)
			}
		})
	}
}

func TestRemoteCompletionProviderOverrideBeatsRecordProvider(t *testing.T) {
	store, _ := newArtifactStoreForTest(completionRecord())
	var resolved string
	service := NewRemoteCompletionService(store, TrackedMaterials{}, NewCompletionReceipts(store, fixedNow),
		func(name string) (CompletionProvider, error) { resolved = name; return &completionProviderFake{}, nil },
		func(model.IssueOpsRemoteArtifactVerification) error { return nil })
	if _, _, err := service.Close(context.Background(), "io-done", " gitlab ", false); err != nil || resolved != "gitlab" {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
}

func TestRemoteCompletionCloseConfirmStampsIssueClosedOnce(t *testing.T) {
	h := newCompletionHarness(completionRecord(), &completionProviderFake{closeResult: port.IssueProviderCloseIssueResult{OK: true, Closed: true}})
	record, result, err := h.service.Close(context.Background(), "io-done", "", true)
	if err != nil || !result.Closed || record.RemoteCompletion == nil || record.RemoteCompletion.IssueClosedAt != "2026-03-04T05:06:07Z" {
		t.Fatalf("record=%+v result=%+v err=%v", record.RemoteCompletion, result, err)
	}
	if req := h.provider.closes[0]; !req.Confirm || req.IssueURL != "https://github.com/acme/repo/issues/4" || req.Repo != "/repo" {
		t.Fatalf("request = %+v", req)
	}
}

func TestRemoteCompletionCloseWithoutClosureLeavesRecordAlone(t *testing.T) {
	for _, tc := range []struct {
		name    string
		confirm bool
		result  port.IssueProviderCloseIssueResult
	}{
		{"preview", false, port.IssueProviderCloseIssueResult{OK: true, Closed: true}},
		{"provider did not close", true, port.IssueProviderCloseIssueResult{OK: true, Closed: false, AlreadyClosed: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCompletionHarness(completionRecord(), &completionProviderFake{closeResult: tc.result})
			record, _, err := h.service.Close(context.Background(), "io-done", "", tc.confirm)
			if err != nil || record.RemoteCompletion != nil || h.store.records["io-done"].RemoteCompletion != nil {
				t.Fatalf("record=%+v err=%v", record, err)
			}
		})
	}
}

func TestRemoteCompletionCloseRefusalsAndProviderFailure(t *testing.T) {
	noIssue := completionRecord()
	noIssue.IssueURL = ""
	noIssue.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github"}
	h := newCompletionHarness(noIssue, &completionProviderFake{})
	if _, _, err := h.service.Close(context.Background(), "io-done", "", true); err == nil || !strings.Contains(err.Error(), "cannot close before a linked issue") || len(h.provider.closes) != 0 {
		t.Fatalf("err = %v", err)
	}
	boom := errors.New("api down")
	h = newCompletionHarness(completionRecord(), &completionProviderFake{closeErr: boom})
	if _, _, err := h.service.Close(context.Background(), "io-done", "", true); !errors.Is(err, boom) || h.store.records["io-done"].RemoteCompletion != nil {
		t.Fatalf("err = %v", err)
	}
	h = newCompletionHarness(completionRecord(), &completionProviderFake{updateErr: boom})
	if _, _, _, err := h.service.Reflect(context.Background(), "io-done", "", completionReport, true); !errors.Is(err, boom) || h.store.records["io-done"].RemoteCompletion != nil {
		t.Fatalf("reflect err = %v", err)
	}
}

// A confirmed write whose provider call failed may still have landed; the
// error says to re-run the same idempotent command. A preview error stays as is.
func TestRemoteCompletionFailedConfirmSaysTheRetryIsIdempotent(t *testing.T) {
	lost := errors.New("context deadline exceeded")
	h := newCompletionHarness(completionRecord(), &completionProviderFake{closeErr: lost, updateErr: lost})
	if _, _, err := h.service.Close(context.Background(), "io-done", "", true); !errors.Is(err, lost) || !strings.Contains(err.Error(), "re-run this same command") || !strings.Contains(err.Error(), "already closed") {
		t.Fatalf("close err = %v", err)
	}
	if _, _, _, err := h.service.Reflect(context.Background(), "io-done", "", completionReport, true); !errors.Is(err, lost) || !strings.Contains(err.Error(), "already reflected") {
		t.Fatalf("reflect err = %v", err)
	}
	if _, _, err := h.service.Close(context.Background(), "io-done", "", false); err == nil || strings.Contains(err.Error(), "re-run") {
		t.Fatalf("preview err = %v", err)
	}
}
