package issueopsremote

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	executionissue "issueops/internal/contract/executionissue"
	model "issueops/internal/contract/issueops"
	bodysyncc "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/port"
)

type posterFake struct {
	url   string
	err   error
	calls []string
}

func (p *posterFake) Post(provider, issueURL, body string) (string, error) {
	p.calls = append(p.calls, provider+"|"+issueURL+"|"+body)
	return p.url, p.err
}

func graphRecord(issueURL, provider string, links ...model.IssueOpsIssueLink) model.IssueOpsRecord {
	record := model.IssueOpsRecord{ID: "io-graph", IssueURL: issueURL, IssueLinks: links}
	if provider != "" {
		record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: provider}
	}
	return record
}

func TestIssueGraphSyncPreviewNeverPostsAndCountsLinks(t *testing.T) {
	poster := &posterFake{url: "unused"}
	record := graphRecord("", "", model.IssueOpsIssueLink{Type: "child", URL: "u1"}, model.IssueOpsIssueLink{Type: "blocks", URL: "u2"})
	store, _ := newArtifactStoreForTest(record)
	got, err := NewIssueGraphSyncService(store, poster).Sync(context.Background(), record.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if got["dry_run"] != true || got["synced"] != false || got["link_count"] != 2 || len(poster.calls) != 0 {
		t.Fatalf("preview = %v, posts = %v", got, poster.calls)
	}
}

func TestIssueGraphSyncConfirmPostsProviderSpecificResult(t *testing.T) {
	links := []model.IssueOpsIssueLink{{Type: "depends-on", URL: "https://x/1", Title: "선행"}, {Type: "child", URL: "https://x/2"}}
	for _, tc := range []struct {
		provider, urlKey string
	}{{"github", "comment_url"}, {"gitlab", "note_url"}} {
		t.Run(tc.provider, func(t *testing.T) {
			poster := &posterFake{url: "https://posted/1"}
			record := graphRecord("https://host/issues/1", tc.provider, links...)
			store, _ := newArtifactStoreForTest(record)
			got, err := NewIssueGraphSyncService(store, poster).Sync(context.Background(), record.ID, true)
			if err != nil {
				t.Fatal(err)
			}
			if got["synced"] != true || got[tc.urlKey] != "https://posted/1" || got["link_count"] != 2 || got["provider"] != tc.provider {
				t.Fatalf("result = %v", got)
			}
			if _, other := got["comment_url"]; tc.provider == "gitlab" && other {
				t.Fatalf("gitlab result must not use comment_url: %v", got)
			}
			if len(poster.calls) != 1 || !strings.HasPrefix(poster.calls[0], tc.provider+"|https://host/issues/1|") || !strings.Contains(poster.calls[0], "선행 이슈") {
				t.Fatalf("post = %v", poster.calls)
			}
		})
	}
}

func TestIssueGraphSyncConfirmRefusalsAndNoop(t *testing.T) {
	link := model.IssueOpsIssueLink{Type: "child", URL: "https://x/2"}
	for _, tc := range []struct {
		name    string
		record  model.IssueOpsRecord
		wantErr string
		noop    bool
	}{
		{"no issue url", graphRecord("", "github", link), "no issue_url", false},
		{"no links", graphRecord("https://host/issues/1", "github"), "", true},
		{"unknown provider", graphRecord("https://host/issues/1", "", link), "cannot determine provider", false},
		{"unsupported provider", graphRecord("https://host/issues/1", "bitbucket", link), "not supported", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			poster := &posterFake{url: "x"}
			store, _ := newArtifactStoreForTest(tc.record)
			got, err := NewIssueGraphSyncService(store, poster).Sync(context.Background(), tc.record.ID, true)
			if len(poster.calls) != 0 {
				t.Fatalf("must not post: %v", poster.calls)
			}
			if tc.noop {
				if err != nil || got["synced"] != false || got["ok"] != true {
					t.Fatalf("noop = %v, %v", got, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestIssueGraphSyncPropagatesReadAndPostFailures(t *testing.T) {
	store, _ := newArtifactStoreForTest(model.IssueOpsRecord{ID: "io-graph"})
	if _, err := NewIssueGraphSyncService(store, &posterFake{}).Sync(context.Background(), "io-missing", true); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read err = %v", err)
	}
	boom := errors.New("post failed")
	record := graphRecord("https://host/issues/1", "github", model.IssueOpsIssueLink{Type: "child", URL: "u"})
	store, _ = newArtifactStoreForTest(record)
	if got, err := NewIssueGraphSyncService(store, &posterFake{err: boom}).Sync(context.Background(), record.ID, true); !errors.Is(err, boom) || got != nil {
		t.Fatalf("post err = %v, result %v", err, got)
	}
}

func TestBodySyncCommandResolvesProviderBodyAndActor(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-sync", IssueURL: "https://github.com/acme/repo/issues/1"}
	store, _ := newArtifactStoreForTest(record)
	ancestry := []model.NativeProcessReceipt{{PID: 42}}
	var gotName, gotID string
	var gotCmd bodysyncc.Command
	var gotActor model.IssueOpsActor
	resolve := func(name string) (BodySyncOperation, error) {
		gotName = name
		return func(_ context.Context, id string, cmd bodysyncc.Command, actor model.IssueOpsActor) (model.IssueOpsRecord, bodysyncc.Result, error) {
			gotID, gotCmd, gotActor = id, cmd, actor
			return record, bodysyncc.Result{}, nil
		}, nil
	}
	bodyFile := t.TempDir() + "/body.md"
	if err := os.WriteFile(bodyFile, []byte("\n새 본문\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewBodySyncCommandService(store, NewTemplateBodyResolver(os.ReadFile), resolve, func() ([]model.NativeProcessReceipt, error) { return ancestry, nil })
	_, _, err := service.Sync(context.Background(), BodySyncInput{BodyFile: bodyFile, Command: bodysyncc.Command{ID: "io-sync", Kind: "issue"}, Actor: model.IssueOpsActor{Host: "codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "github" || gotID != "io-sync" || gotCmd.ProposedBody != "새 본문" || gotActor.Host != "codex" || len(gotActor.NativeProcessAncestry) != 1 || gotActor.NativeProcessAncestry[0].PID != 42 {
		t.Fatalf("name=%q id=%q cmd=%+v actor=%+v", gotName, gotID, gotCmd, gotActor)
	}
}

func TestBodySyncCommandRefusesBeforeOperationRuns(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-sync", IssueURL: "https://github.com/acme/repo/issues/1"}
	noProvider := model.IssueOpsRecord{ID: "io-noprov"}
	store, _ := newArtifactStoreForTest(record)
	store.records[noProvider.ID] = noProvider
	ran := false
	op := func(context.Context, string, bodysyncc.Command, model.IssueOpsActor) (model.IssueOpsRecord, bodysyncc.Result, error) {
		ran = true
		return record, bodysyncc.Result{}, nil
	}
	observeOK := func() ([]model.NativeProcessReceipt, error) { return nil, nil }
	resolveOK := func(string) (BodySyncOperation, error) { return op, nil }
	bodies := NewTemplateBodyResolver(os.ReadFile)
	boom := errors.New("boom")
	secret := "ghp_" + strings.Repeat("a", 36)
	for _, tc := range []struct {
		name    string
		id      string
		input   BodySyncInput
		resolve BodySyncResolver
		observe AncestryObserver
		check   func(error) bool
	}{
		{"missing record", "io-missing", BodySyncInput{}, resolveOK, observeOK, func(err error) bool { return errors.Is(err, os.ErrNotExist) }},
		{"no provider", "io-noprov", BodySyncInput{}, resolveOK, observeOK, func(err error) bool { return strings.Contains(err.Error(), "cannot determine provider") }},
		{"provider resolve fails", "io-sync", BodySyncInput{}, func(string) (BodySyncOperation, error) { return nil, boom }, observeOK, func(err error) bool { return errors.Is(err, boom) }},
		{"body and file", "io-sync", BodySyncInput{BodyFile: "x", Command: bodysyncc.Command{ProposedBody: "y"}}, resolveOK, observeOK, func(err error) bool { return strings.Contains(err.Error(), "mutually exclusive") }},
		{"empty body", "io-sync", BodySyncInput{}, resolveOK, observeOK, func(err error) bool { return strings.Contains(err.Error(), "replacement body is required") }},
		{"secret body", "io-sync", BodySyncInput{Command: bodysyncc.Command{Kind: "issue", ProposedBody: secret}}, resolveOK, observeOK, func(err error) bool { return strings.Contains(err.Error(), "secret-like") }},
		{"ancestry fails", "io-sync", BodySyncInput{Command: bodysyncc.Command{ProposedBody: "본문"}}, resolveOK, func() ([]model.NativeProcessReceipt, error) { return nil, boom }, func(err error) bool { return errors.Is(err, boom) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.input
			input.Command.ID = tc.id
			_, _, err := NewBodySyncCommandService(store, bodies, tc.resolve, tc.observe).Sync(context.Background(), input)
			if err == nil || !tc.check(err) {
				t.Fatalf("err = %v", err)
			}
			if ran {
				t.Fatal("operation must not run after a refusal")
			}
		})
	}
}

func publicationRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "io-pub", Branch: "7-demo", IssueURL: "https://github.com/acme/repo/issues/7", BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main"}}
}

const koreanBody = "## 요약\n\n보고서 형식을 정리했습니다.\n\n## 변경 내용\n\n보고서 형식을 한국어로 정리했습니다.\n\n## 확인한 것\n\n단위 시험을 통과했습니다.\n\n## 리뷰 포인트\n\n본문 구성을 봐 주세요.\n"

func TestPublicationCommandAppliesDefaultsAndNormalizesActorOnConfirm(t *testing.T) {
	record := publicationRecord()
	store, _ := newArtifactStoreForTest(record)
	var got model.RemotePullRequestRequest
	publish := func(_ context.Context, req model.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
		got = req
		return executionissue.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/acme/repo/pull/9"}, nil
	}
	ancestry := []model.NativeProcessReceipt{{PID: 7}}
	normalize := func(_ context.Context, actor model.NativeActor) (model.NativeActor, error) {
		if len(actor.ProcessAncestry) != 1 {
			t.Fatalf("normalize must see observed ancestry: %+v", actor)
		}
		actor.AgentID = "normalized"
		return actor, nil
	}
	service := NewPublicationCommandService(store, NewTemplateBodyResolver(os.ReadFile), func() ([]model.NativeProcessReceipt, error) { return ancestry, nil }, normalize, publish)
	result, err := service.Create(context.Background(), PublicationInput{Request: model.RemotePullRequestRequest{ID: "io-pub", Title: "변경 제목", Body: koreanBody, Labels: []string{" a ", ""}, Assignees: []string{"me"}, Confirm: true}})
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://github.com/acme/repo/pull/9" || got.Provider != "github" || got.Head != "7-demo" || got.Base != "main" || got.Actor.AgentID != "normalized" {
		t.Fatalf("result=%+v request=%+v", result, got)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "a" {
		t.Fatalf("labels must be cleaned: %v", got.Labels)
	}
}

func TestPublicationCommandPreviewSkipsActorWork(t *testing.T) {
	record := publicationRecord()
	store, _ := newArtifactStoreForTest(record)
	published := 0
	publish := func(context.Context, model.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
		published++
		return executionissue.IssueProviderCreatePullRequestResult{OK: true, Preview: "preview"}, nil
	}
	fail := func() ([]model.NativeProcessReceipt, error) {
		t.Fatal("preview must not observe ancestry")
		return nil, nil
	}
	service := NewPublicationCommandService(store, NewTemplateBodyResolver(os.ReadFile), fail, nil, publish)
	result, err := service.Create(context.Background(), PublicationInput{Request: model.RemotePullRequestRequest{ID: "io-pub", Title: "제목", Body: koreanBody}})
	if err != nil || result.Preview != "preview" || published != 1 {
		t.Fatalf("result=%+v err=%v published=%d", result, err, published)
	}
}

func TestPublicationCommandRefusals(t *testing.T) {
	boom := errors.New("boom")
	okPublish := func(context.Context, model.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
		return executionissue.IssueProviderCreatePullRequestResult{}, nil
	}
	okObserve := func() ([]model.NativeProcessReceipt, error) { return nil, nil }
	okNormalize := func(_ context.Context, a model.NativeActor) (model.NativeActor, error) { return a, nil }
	confirmReq := model.RemotePullRequestRequest{ID: "io-pub", Title: "제목", Body: koreanBody, Labels: []string{"l"}, Assignees: []string{"a"}, Confirm: true}
	secret := "ghp_" + strings.Repeat("a", 36)
	for _, tc := range []struct {
		name      string
		record    model.IssueOpsRecord
		req       model.RemotePullRequestRequest
		observe   AncestryObserver
		normalize func(context.Context, model.NativeActor) (model.NativeActor, error)
		publish   PublicationInvoker
		want      func(error) bool
	}{
		{"missing record", publicationRecord(), model.RemotePullRequestRequest{ID: "io-missing"}, okObserve, okNormalize, okPublish, func(err error) bool { return errors.Is(err, os.ErrNotExist) }},
		{"no provider", model.IssueOpsRecord{ID: "io-pub"}, model.RemotePullRequestRequest{ID: "io-pub"}, okObserve, okNormalize, okPublish, func(err error) bool { return strings.Contains(err.Error(), "cannot determine provider") }},
		{"body and file conflict", publicationRecord(), model.RemotePullRequestRequest{ID: "io-pub", Body: "x"}, okObserve, okNormalize, okPublish, nil},
		{"secret title", publicationRecord(), model.RemotePullRequestRequest{ID: "io-pub", Title: secret, Body: koreanBody}, okObserve, okNormalize, okPublish, func(err error) bool { return strings.Contains(err.Error(), "secret-like") }},
		{"confirm without labels", publicationRecord(), model.RemotePullRequestRequest{ID: "io-pub", Title: "제목", Body: koreanBody, Confirm: true}, okObserve, okNormalize, okPublish, func(err error) bool { return strings.Contains(err.Error(), "label is required") }},
		{"ancestry fails", publicationRecord(), confirmReq, func() ([]model.NativeProcessReceipt, error) { return nil, boom }, okNormalize, okPublish, func(err error) bool { return errors.Is(err, boom) }},
		{"handler unavailable", publicationRecord(), confirmReq, okObserve, okNormalize, nil, func(err error) bool { return errors.Is(err, model.ErrRemotePullRequestCreateHandlerUnavailable) }},
		{"normalize fails", publicationRecord(), confirmReq, okObserve, func(context.Context, model.NativeActor) (model.NativeActor, error) { return model.NativeActor{}, boom }, okPublish, func(err error) bool { return errors.Is(err, boom) }},
		{"publish fails", publicationRecord(), confirmReq, okObserve, okNormalize, func(context.Context, model.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
			return executionissue.IssueProviderCreatePullRequestResult{}, boom
		}, func(err error) bool { return errors.Is(err, boom) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newArtifactStoreForTest(tc.record)
			input := PublicationInput{Request: tc.req}
			if tc.name == "body and file conflict" {
				input.BodyFile = "some-file"
			}
			_, err := NewPublicationCommandService(store, NewTemplateBodyResolver(os.ReadFile), tc.observe, tc.normalize, tc.publish).Create(context.Background(), input)
			if err == nil {
				t.Fatal("expected refusal")
			}
			if tc.want != nil && !tc.want(err) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

type reviewProviderFake struct {
	result   port.IssueProviderUpdateIssueBodySectionResult
	err      error
	requests []port.IssueProviderUpdateIssueBodySectionRequest
}

func (p *reviewProviderFake) UpdateIssueBodySection(_ context.Context, req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	p.requests = append(p.requests, req)
	return p.result, p.err
}

type authorityFake struct {
	err   error
	calls int
}

func (a *authorityFake) Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error {
	a.calls++
	return a.err
}

func reviewedRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID: "io-review", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/3",
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{
			Verdict: "approve", Findings: []string{"f1", "f2"},
			History: []model.IssueOpsDevilsAdvocateRound{{Verdict: "revise", Findings: []string{"old"}}},
		},
	}
}

func newReviewService(store ReviewReflectionStore, auth Authority, provider ReviewReflectionProvider, now time.Time) *ReviewReflectionService {
	return NewReviewReflectionService(store, auth, func(string) (ReviewReflectionProvider, error) { return provider, nil }, func() ([]model.NativeProcessReceipt, error) { return nil, nil }, func() time.Time { return now })
}

func TestReviewReflectionConfirmStampsRecordAfterRemoteUpdate(t *testing.T) {
	store, _ := newArtifactStoreForTest(reviewedRecord())
	provider := &reviewProviderFake{result: port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: true}}
	auth := &authorityFake{}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	record, result, err := newReviewService(store, auth, provider, now).Reflect(context.Background(), "io-review", "", true, model.IssueOpsActor{})
	if err != nil || !result.Updated {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if record.DevilsAdvocateReview.IssueReflectedAt != "2026-01-02T03:04:05Z" || store.records["io-review"].DevilsAdvocateReview.IssueReflectedAt == "" {
		t.Fatalf("stamp not persisted: %+v", store.records["io-review"].DevilsAdvocateReview)
	}
	req := provider.requests[0]
	if !req.Confirm || req.Section != model.IssueBodySectionDevilsAdvocate || len(req.Rounds) != 2 || req.Rounds[0].Findings != 1 || req.Rounds[1].Verdict != "approve" || req.Rounds[1].Findings != 2 {
		t.Fatalf("request = %+v", req)
	}
	if auth.calls != 2 {
		t.Fatalf("authority must be checked before the remote write and again inside the stamp transaction, got %d", auth.calls)
	}
}

func TestReviewReflectionPreviewAndNoopLeaveRecordUnstamped(t *testing.T) {
	for _, tc := range []struct {
		name    string
		result  port.IssueProviderUpdateIssueBodySectionResult
		confirm bool
	}{
		{"preview", port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: true}, false},
		{"already up to date", port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: false}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newArtifactStoreForTest(reviewedRecord())
			auth := &authorityFake{}
			_, _, err := newReviewService(store, auth, &reviewProviderFake{result: tc.result}, time.Now()).Reflect(context.Background(), "io-review", "", tc.confirm, model.IssueOpsActor{})
			if err != nil {
				t.Fatal(err)
			}
			if store.records["io-review"].DevilsAdvocateReview.IssueReflectedAt != "" || auth.calls != 1 {
				t.Fatalf("record must stay unstamped, auth calls=%d", auth.calls)
			}
		})
	}
}

func TestReviewReflectionRefusals(t *testing.T) {
	boom := errors.New("boom")
	noFindings := reviewedRecord()
	noFindings.DevilsAdvocateReview.Findings = nil
	noIssue := reviewedRecord()
	noIssue.IssueURL = ""
	noIssue.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github"}
	noProvider := reviewedRecord()
	noProvider.IssueURL = "https://example.com/issues/3"
	ok := func() ([]model.NativeProcessReceipt, error) { return nil, nil }
	for _, tc := range []struct {
		name     string
		id       string
		record   model.IssueOpsRecord
		auth     error
		resolve  ReviewProviderResolver
		observe  AncestryObserver
		provider *reviewProviderFake
		want     string
	}{
		{name: "missing record", id: "io-missing", record: reviewedRecord(), want: "file does not exist"},
		{name: "no provider", id: "io-review", record: noProvider, want: "cannot determine provider"},
		{name: "resolver fails", id: "io-review", record: reviewedRecord(), resolve: func(string) (ReviewReflectionProvider, error) { return nil, boom }, want: "boom"},
		{name: "nil provider", id: "io-review", record: reviewedRecord(), resolve: func(string) (ReviewReflectionProvider, error) { return nil, nil }, want: "no issue provider configured"},
		{name: "ancestry fails", id: "io-review", record: reviewedRecord(), observe: func() ([]model.NativeProcessReceipt, error) { return nil, boom }, want: "boom"},
		{name: "authority denies", id: "io-review", record: reviewedRecord(), auth: boom, want: "boom"},
		{name: "no findings", id: "io-review", record: noFindings, want: "no devil's-advocate findings"},
		{name: "no linked issue", id: "io-review", record: noIssue, want: "cannot reflect findings before a linked issue"},
		{name: "provider update fails", id: "io-review", record: reviewedRecord(), provider: &reviewProviderFake{err: boom}, want: "boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newArtifactStoreForTest(tc.record)
			provider := tc.provider
			if provider == nil {
				provider = &reviewProviderFake{}
			}
			resolve, observe := tc.resolve, tc.observe
			if resolve == nil {
				resolve = func(string) (ReviewReflectionProvider, error) { return provider, nil }
			}
			if observe == nil {
				observe = ok
			}
			service := NewReviewReflectionService(store, &authorityFake{err: tc.auth}, resolve, observe, time.Now)
			_, _, err := service.Reflect(context.Background(), tc.id, "", true, model.IssueOpsActor{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if tc.provider == nil && len(provider.requests) != 0 {
				t.Fatalf("remote must not be touched after refusal: %+v", provider.requests)
			}
		})
	}
}

func TestReviewReflectionStampFailureAfterRemoteUpdate(t *testing.T) {
	store := &vanishingReviewStore{artifactStoreForTest: &artifactStoreForTest{records: map[string]model.IssueOpsRecord{"io-review": reviewedRecord()}}}
	provider := &reviewProviderFake{result: port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: true}}
	_, result, err := newReviewService(store, &authorityFake{}, provider, time.Now()).Reflect(context.Background(), "io-review", "", true, model.IssueOpsActor{})
	if err == nil || !strings.Contains(err.Error(), "disappeared before reflect stamp") || !result.Updated {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

// vanishingReviewStore drops the review between the pre-write read and the
// stamp transaction, as a concurrent writer could.
type vanishingReviewStore struct {
	*artifactStoreForTest
}

func (s *vanishingReviewStore) Update(ctx context.Context, id string, transition RecordTransition) (model.IssueOpsRecord, error) {
	record := s.records[id]
	record.DevilsAdvocateReview = nil
	s.records[id] = record
	return s.artifactStoreForTest.Update(ctx, id, transition)
}

// QA: an issue closed as a duplicate between sessions went unnoticed and the
// next session published a PR that "Closes" it. The preview names the state.
func TestPublicationPreviewWarnsWhenTheIssueIsClosed(t *testing.T) {
	record := publicationRecord()
	publish := func(context.Context, model.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
		return executionissue.IssueProviderCreatePullRequestResult{OK: true, Preview: "preview"}, nil
	}
	for _, tc := range []struct {
		state string
		err   error
		want  string
	}{{"CLOSED", nil, "issue_closed: "}, {"closed", nil, "issue_closed: "}, {"OPEN", nil, ""}, {"", errors.New("gh down"), "issue_state_unread: "}} {
		store, _ := newArtifactStoreForTest(record)
		service := NewPublicationCommandService(store, NewTemplateBodyResolver(os.ReadFile), nil, nil, publish).
			WithIssueState(func(context.Context, model.IssueOpsRecord, string) (string, error) { return tc.state, tc.err })
		result, err := service.Create(context.Background(), PublicationInput{Request: model.RemotePullRequestRequest{ID: "io-pub", Title: "제목", Body: koreanBody}})
		if err != nil {
			t.Fatal(err)
		}
		if tc.want == "" && len(result.Warnings) != 0 || tc.want != "" && (len(result.Warnings) != 1 || !strings.HasPrefix(result.Warnings[0], tc.want)) {
			t.Fatalf("state %q: warnings %q", tc.state, result.Warnings)
		}
	}
}

// QA: a body with only the issueops sections was published in a repository
// whose PR template asked for What, Risk and a checklist.
func TestPublicationPreviewNamesRepositoryTemplateSectionsTheBodyLacks(t *testing.T) {
	record := publicationRecord()
	record.Repo, record.WorktreePath = "/repo", "/repo.worktrees/7-demo"
	publish := func(context.Context, model.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
		return executionissue.IssueProviderCreatePullRequestResult{OK: true, Preview: "preview"}, nil
	}
	files := map[string]string{}
	read := func(path string) ([]byte, error) {
		if content, ok := files[path]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}
	root := record.WorktreePath
	create := func() []string {
		store, _ := newArtifactStoreForTest(record)
		service := NewPublicationCommandService(store, NewTemplateBodyResolver(os.ReadFile), nil, nil, publish).WithRepoTemplates(read)
		result, err := service.Create(context.Background(), PublicationInput{Request: model.RemotePullRequestRequest{ID: "io-pub", Title: "제목", Body: koreanBody}})
		if err != nil {
			t.Fatal(err)
		}
		return result.Warnings
	}
	if got := create(); len(got) != 0 {
		t.Fatalf("no repository template must not warn: %q", got)
	}
	files[root+"/.github/pull_request_template.md"] = "## What\n\n## Risk\n"
	if got := create(); len(got) != 1 || !strings.HasPrefix(got[0], "repo_template_sections_missing: .github/pull_request_template.md asks for sections the body lacks: What, Risk.") {
		t.Fatalf("warnings = %q", got)
	}
}
