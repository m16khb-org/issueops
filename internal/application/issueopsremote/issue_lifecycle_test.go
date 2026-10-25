package issueopsremote

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

const (
	testAuthority = "github.com/acme/repo"
	testIssueURL  = "https://github.com/acme/repo/issues/5"
)

var fixedNow = func() time.Time { return time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC) }

var issueFields = []string{
	"summary=이슈 본문을 요약부터 적습니다.",
	"background=지금 본문은 읽기 어렵습니다.",
	"acceptance=새 본문의 첫 절이 요약입니다.",
	"scope=테스트 대상 패키지만 바꿉니다.",
	"verification=go test 로 확인합니다.",
}

type createEnv struct {
	inferred     string
	inferErr     error
	authority    string
	authorityErr error
	resolveErr   error
	invoke       IssueCreateInvoker
	inferCalls   int
}

func (e *createEnv) InferProvider(string) (string, error) {
	e.inferCalls++
	return e.inferred, e.inferErr
}
func (e *createEnv) ProjectAuthority(string, string) (string, error) {
	return e.authority, e.authorityErr
}
func (e *createEnv) Resolve(string) (IssueCreateInvoker, error) { return e.invoke, e.resolveErr }

type createHarness struct {
	store    *artifactStoreForTest
	env      *createEnv
	requests []port.IssueProviderCreateIssueRequest
	ctxErrs  []error
	verified []model.IssueOpsRemoteArtifactVerificationRequest
	verifyFn func(model.IssueOpsRemoteArtifactVerificationRequest) error
	creator  *IssueCreator
}

func newCreateHarness(t *testing.T, result port.IssueProviderCreateIssueResult, createErr error) *createHarness {
	t.Helper()
	record := model.IssueOpsRecord{ID: "io-create", Repo: "/repo"}
	store, _ := newArtifactStoreForTest(record)
	h := &createHarness{store: store}
	h.env = &createEnv{inferred: "github", authority: testAuthority}
	h.env.invoke = func(ctx context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
		h.requests = append(h.requests, req)
		h.ctxErrs = append(h.ctxErrs, ctx.Err())
		return result, createErr
	}
	verify := func(_ context.Context, req model.IssueOpsRemoteArtifactVerificationRequest) error {
		h.verified = append(h.verified, req)
		if h.verifyFn != nil {
			return h.verifyFn(req)
		}
		return nil
	}
	h.creator = NewIssueCreator(store, h.env, NewTemplateBodyResolver(os.ReadFile), NewIssueCreateIntents(store, fixedNow), verify, fixedNow)
	return h
}

func confirmCmd() IssueCreateCommand {
	return IssueCreateCommand{ID: "io-create", Title: "이슈 제목", Template: "implementation_task", Fields: issueFields, Labels: []string{" bug "}, Assignees: []string{"me"}, Confirm: true}
}

func (h *createHarness) intent() *model.IssueOpsIssueCreateIntent {
	return h.store.records["io-create"].IssueCreateIntent
}

func TestIssueCreatePreviewInvokesProviderWithoutSealingIntent(t *testing.T) {
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{OK: true, Preview: "dry"}, nil)
	cmd := confirmCmd()
	cmd.Confirm, cmd.Template = false, ""
	cmd.Labels = []string{" bug ", ""}
	result, err := h.creator.Create(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if h.intent() != nil || len(h.verified) != 0 {
		t.Fatalf("preview must not seal an intent or verify: %+v", h.intent())
	}
	req := h.requests[0]
	if req.Confirm || req.ProjectKey != "" || strings.Contains(req.Body, "issueops:issue-create") {
		t.Fatalf("preview request = %+v", req)
	}
	if result.Preview != "dry" || result.Provider != "github" || len(result.Labels) != 1 || result.Labels[0] != "bug" || h.env.inferCalls != 1 {
		t.Fatalf("result = %+v inferCalls=%d", result, h.env.inferCalls)
	}
}

// A preview that passes and then fails at --confirm on an unresolvable origin
// hides the one problem it exists to show.
func TestIssueCreatePreviewFailsWhereConfirmWouldOnProjectAuthority(t *testing.T) {
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{OK: true, Preview: "dry"}, nil)
	h.env.authorityErr = errors.New("cannot determine project authority: git remote URL must be HTTPS, SSH, or SCP syntax")
	cmd := confirmCmd()
	cmd.Confirm, cmd.Template = false, ""
	if _, err := h.creator.Create(context.Background(), cmd); err == nil || !strings.Contains(err.Error(), "project authority") {
		t.Fatalf("err = %v", err)
	}
	if len(h.requests) != 0 {
		t.Fatalf("provider was invoked: %+v", h.requests)
	}
}

func TestIssueCreateConfirmSealsIntentAndCompletesAfterLiveVerification(t *testing.T) {
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{OK: true, URL: testIssueURL, Number: "5"}, nil)
	result, err := h.creator.Create(context.Background(), confirmCmd())
	if err != nil {
		t.Fatal(err)
	}
	req := h.requests[0]
	if !req.Confirm || req.ProjectKey != testAuthority || !strings.Contains(req.Body, "<!-- issueops:issue-create:") {
		t.Fatalf("request = %+v", req)
	}
	intent := h.intent()
	if intent == nil || intent.Status != model.IssueCreateIntentCompleted || intent.CanonicalURL != testIssueURL || intent.ProjectAuthority != testAuthority {
		t.Fatalf("intent = %+v", intent)
	}
	if sum := sha256.Sum256([]byte(req.Body)); intent.BodySHA256 != fmt.Sprintf("%x", sum[:]) {
		t.Fatalf("sealed digest must cover the exact body sent to the provider")
	}
	if got := h.store.records["io-create"]; got.IssueURL != testIssueURL || got.UpdatedAt != "2026-03-04T05:06:07Z" {
		t.Fatalf("record = %+v", got)
	}
	if len(h.verified) != 1 || h.verified[0].Kind != "issue" || h.verified[0].URL != testIssueURL || h.verified[0].Provider != "github" {
		t.Fatalf("verification = %+v", h.verified)
	}
	if result.URL != testIssueURL || !result.Readability.OK {
		t.Fatalf("result = %+v", result)
	}
}

func TestIssueCreateConfirmCallsProviderUnderCallerContextButPersistsReceiptAfterCancellation(t *testing.T) {
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{OK: true, URL: testIssueURL}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.creator.Create(ctx, confirmCmd()); err != nil {
		t.Fatal(err)
	}
	if h.ctxErrs[0] == nil {
		t.Fatal("a confirmed create must run under the caller's context")
	}
	if h.intent().Status != model.IssueCreateIntentCompleted {
		t.Fatalf("receipt must persist despite cancellation: %+v", h.intent())
	}
}

func TestIssueCreateRecordsFailureClassOnProviderError(t *testing.T) {
	boom := errors.New("gh exploded")
	for _, tc := range []struct {
		name   string
		err    error
		result port.IssueProviderCreateIssueResult
		status string
	}{
		{"not invoked", &port.IssueProviderCreateError{Invoked: false, Err: boom}, port.IssueProviderCreateIssueResult{}, model.IssueCreateIntentNotInvoked},
		{"invoked unknown", &port.IssueProviderCreateError{Invoked: true, Err: boom}, port.IssueProviderCreateIssueResult{}, model.IssueCreateIntentInvokedUnknown},
		{"untyped error", boom, port.IssueProviderCreateIssueResult{}, model.IssueCreateIntentInvokedUnknown},
		{"url observed", boom, port.IssueProviderCreateIssueResult{URL: testIssueURL}, model.IssueCreateIntentURLObserved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCreateHarness(t, tc.result, tc.err)
			_, err := h.creator.Create(context.Background(), confirmCmd())
			if !errors.Is(err, boom) {
				t.Fatalf("err = %v", err)
			}
			intent := h.intent()
			if intent == nil || intent.Status != tc.status || !strings.Contains(intent.Failure, "gh exploded") {
				t.Fatalf("intent = %+v, want status %s", intent, tc.status)
			}
			if h.store.records["io-create"].IssueURL != "" {
				t.Fatal("a failed create must not link an issue")
			}
		})
	}
}

func TestIssueCreateProviderErrorInPreviewLeavesRecordUntouched(t *testing.T) {
	boom := errors.New("preview failed")
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{}, boom)
	cmd := confirmCmd()
	cmd.Confirm = false
	if _, err := h.creator.Create(context.Background(), cmd); !errors.Is(err, boom) || h.intent() != nil {
		t.Fatalf("err=%v intent=%+v", err, h.intent())
	}
}

func TestIssueCreateRecordsVerificationFailureWithObservedURL(t *testing.T) {
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{OK: true, URL: testIssueURL}, nil)
	boom := errors.New("label drift")
	h.verifyFn = func(model.IssueOpsRemoteArtifactVerificationRequest) error { return boom }
	if _, err := h.creator.Create(context.Background(), confirmCmd()); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	intent := h.intent()
	if intent.Status != model.IssueCreateIntentVerificationFailed || intent.CanonicalURL != testIssueURL || h.store.records["io-create"].IssueURL != "" {
		t.Fatalf("intent = %+v record URL = %q", intent, h.store.records["io-create"].IssueURL)
	}
}

func TestIssueCreateJoinsReceiptFailureWhenForeignProjectURLIsReturned(t *testing.T) {
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{OK: true, URL: "https://github.com/other/repo/issues/9"}, nil)
	_, err := h.creator.Create(context.Background(), confirmCmd())
	if err == nil || !strings.Contains(err.Error(), "does not match sealed authority") {
		t.Fatalf("err = %v", err)
	}
	if intent := h.intent(); intent.Status != model.IssueCreateIntentPending || h.store.records["io-create"].IssueURL != "" {
		t.Fatalf("a foreign-project URL must neither complete nor be stored: %+v", intent)
	}
}

func TestIssueCreateRefusesInvalidInputsBeforeProvider(t *testing.T) {
	boom := errors.New("boom")
	secret := "ghp_" + strings.Repeat("a", 36)
	for _, tc := range []struct {
		name   string
		mutate func(*IssueCreateCommand, *createEnv)
		want   string
	}{
		{"missing record", func(c *IssueCreateCommand, _ *createEnv) { c.ID = "io-missing" }, "file does not exist"},
		{"blank title", func(c *IssueCreateCommand, _ *createEnv) { c.Title = "  " }, "issue title is required"},
		{"provider inference fails", func(_ *IssueCreateCommand, e *createEnv) { e.inferred, e.inferErr = "", boom }, "boom"},
		{"provider resolve fails", func(_ *IssueCreateCommand, e *createEnv) { e.resolveErr = boom }, "boom"},
		{"confirm needs template", func(c *IssueCreateCommand, _ *createEnv) { c.Template = "" }, "--template is required"},
		{"secret in body", func(c *IssueCreateCommand, _ *createEnv) { c.Body = secret; c.Fields = nil; c.Confirm = false }, "secret-like"},
		{"confirm needs labels", func(c *IssueCreateCommand, _ *createEnv) { c.Labels = nil }, "label is required"},
		{"confirm needs assignees", func(c *IssueCreateCommand, _ *createEnv) { c.Assignees = []string{" "} }, "assignee is required"},
		{"authority lookup fails", func(_ *IssueCreateCommand, e *createEnv) { e.authorityErr = boom }, "boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCreateHarness(t, port.IssueProviderCreateIssueResult{URL: testIssueURL}, nil)
			cmd := confirmCmd()
			tc.mutate(&cmd, h.env)
			_, err := h.creator.Create(context.Background(), cmd)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if len(h.requests) != 0 || h.intent() != nil {
				t.Fatalf("provider/intent must be untouched: requests=%d intent=%+v", len(h.requests), h.intent())
			}
		})
	}
}

func TestIssueCreateRefusesWhenIssueAlreadyLinked(t *testing.T) {
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{URL: testIssueURL}, nil)
	record := h.store.records["io-create"]
	record.IssueURL = "https://github.com/acme/repo/issues/1"
	h.store.records["io-create"] = record
	_, err := h.creator.Create(context.Background(), confirmCmd())
	if err == nil || !strings.Contains(err.Error(), "already links issue") || len(h.requests) != 0 {
		t.Fatalf("err=%v requests=%d", err, len(h.requests))
	}
}

// strandedIntent leaves an invoked_unknown intent behind and returns the body
// the provider received, which the reconciler must match by digest.
func strandedIntent(t *testing.T) (*createHarness, string) {
	t.Helper()
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{}, &port.IssueProviderCreateError{Invoked: true, Err: errors.New("timeout")})
	if _, err := h.creator.Create(context.Background(), confirmCmd()); err == nil {
		t.Fatal("expected provider failure")
	}
	return h, h.requests[0].Body
}

type candidateFake struct {
	result port.IssueProviderFindIssueCreateCandidatesResult
	err    error
	reqs   []port.IssueProviderFindIssueCreateCandidatesRequest
}

func (c *candidateFake) Find(_ context.Context, provider string, req port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error) {
	c.reqs = append(c.reqs, req)
	return c.result, c.err
}

func newReconciler(h *createHarness, candidates IssueCandidateSource) *IssueReconciler {
	return NewIssueReconciler(h.store, candidates, NewIssueCreateIntents(h.store, fixedNow), func(ctx context.Context, req model.IssueOpsRemoteArtifactVerificationRequest) error {
		h.verified = append(h.verified, req)
		if h.verifyFn != nil {
			return h.verifyFn(req)
		}
		return nil
	}, fixedNow)
}

func TestIssueReconcilePreviewReportsAdoptionWithoutMutation(t *testing.T) {
	h, body := strandedIntent(t)
	finder := &candidateFake{result: port.IssueProviderFindIssueCreateCandidatesResult{Candidates: []port.IssueProviderIssueCreateCandidate{{URL: testIssueURL, Title: "이슈 제목", Body: body}}}}
	result, err := newReconciler(h, finder).Reconcile(context.Background(), "io-create", false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.WouldAdopt || result.CandidateURL != testIssueURL || result.CandidateCount != 1 || result.IssueURL != "" {
		t.Fatalf("result = %+v", result)
	}
	if result.NextCommand != "issueops remote reconcile-issue --id io-create --confirm --json" {
		t.Fatalf("preview next_command = %q", result.NextCommand)
	}
	if h.intent().Status != model.IssueCreateIntentInvokedUnknown || len(h.verified) != 0 {
		t.Fatalf("preview must not mutate or verify: %+v", h.intent())
	}
	if finder.reqs[0].Marker != h.intent().Marker || finder.reqs[0].ProjectAuthority != testAuthority || finder.reqs[0].Repo != "/repo" {
		t.Fatalf("search request = %+v", finder.reqs[0])
	}
}

func TestIssueReconcileConfirmAdoptsVerifiedCandidate(t *testing.T) {
	h, body := strandedIntent(t)
	finder := &candidateFake{result: port.IssueProviderFindIssueCreateCandidatesResult{Candidates: []port.IssueProviderIssueCreateCandidate{{URL: testIssueURL, Title: "이슈 제목", Body: body}}}}
	result, err := newReconciler(h, finder).Reconcile(context.Background(), "io-create", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.WouldAdopt || result.NextCommand != "" || result.IssueURL != testIssueURL || result.IssueCreateIntent.Status != model.IssueCreateIntentCompleted {
		t.Fatalf("result = %+v", result)
	}
	if h.store.records["io-create"].IssueURL != testIssueURL {
		t.Fatal("confirmed reconcile must link the adopted issue")
	}
	if len(h.verified) != 1 || len(h.verified[0].Labels) != 1 || h.verified[0].Labels[0] != "bug" || h.verified[0].Assignees[0] != "me" {
		t.Fatalf("verification must use the sealed labels/assignees: %+v", h.verified)
	}
}

func TestIssueReconcileRefusals(t *testing.T) {
	boom := errors.New("search failed")
	cand := func(url, title, body string) port.IssueProviderIssueCreateCandidate {
		return port.IssueProviderIssueCreateCandidate{URL: url, Title: title, Body: body}
	}
	for _, tc := range []struct {
		name   string
		result func(body string) port.IssueProviderFindIssueCreateCandidatesResult
		err    error
		want   string
	}{
		{"search fails", func(string) port.IssueProviderFindIssueCreateCandidatesResult {
			return port.IssueProviderFindIssueCreateCandidatesResult{}
		}, boom, "search failed"},
		{"truncated", func(string) port.IssueProviderFindIssueCreateCandidatesResult {
			return port.IssueProviderFindIssueCreateCandidatesResult{Truncated: true, Candidates: []port.IssueProviderIssueCreateCandidate{cand(testIssueURL, "", "")}}
		}, nil, "truncated"},
		{"no candidates", func(string) port.IssueProviderFindIssueCreateCandidatesResult {
			return port.IssueProviderFindIssueCreateCandidatesResult{}
		}, nil, "found 0 live candidates"},
		{"ambiguous", func(body string) port.IssueProviderFindIssueCreateCandidatesResult {
			return port.IssueProviderFindIssueCreateCandidatesResult{Candidates: []port.IssueProviderIssueCreateCandidate{cand(testIssueURL, "이슈 제목", body), cand(testIssueURL, "이슈 제목", body)}}
		}, nil, "found 2 live candidates"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, body := strandedIntent(t)
			_, err := newReconciler(h, &candidateFake{result: tc.result(body), err: tc.err}).Reconcile(context.Background(), "io-create", true)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if h.intent().Status != model.IssueCreateIntentInvokedUnknown {
				t.Fatalf("indeterminate search must leave the intent untouched: %+v", h.intent())
			}
		})
	}
}

func TestIssueReconcileRefusesMissingOrCompletedIntentAndUnreadableRecord(t *testing.T) {
	h := newCreateHarness(t, port.IssueProviderCreateIssueResult{URL: testIssueURL}, nil)
	finder := &candidateFake{}
	if _, err := newReconciler(h, finder).Reconcile(context.Background(), "io-create", true); err == nil || !strings.Contains(err.Error(), "no issue create intent") {
		t.Fatalf("no intent err = %v", err)
	}
	if _, err := newReconciler(h, finder).Reconcile(context.Background(), "io-missing", true); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing record err = %v", err)
	}
	if _, err := h.creator.Create(context.Background(), confirmCmd()); err != nil {
		t.Fatal(err)
	}
	if _, err := newReconciler(h, finder).Reconcile(context.Background(), "io-create", true); err == nil || !strings.Contains(err.Error(), "already completed") || len(finder.reqs) != 0 {
		t.Fatalf("completed err = %v searches=%d", err, len(finder.reqs))
	}
}

func TestIssueReconcileRecordsMismatchOnlyWhenConfirmed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cand     func(body string) port.IssueProviderIssueCreateCandidate
		confirm  bool
		wantErr  string
		status   string
		wantURL  string
		joinsOut bool
	}{
		{"title drift preview", func(b string) port.IssueProviderIssueCreateCandidate {
			return port.IssueProviderIssueCreateCandidate{URL: testIssueURL, Title: "다른 제목", Body: b}
		}, false, "sealed title and body digest", model.IssueCreateIntentInvokedUnknown, "", false},
		{"title drift confirmed", func(b string) port.IssueProviderIssueCreateCandidate {
			return port.IssueProviderIssueCreateCandidate{URL: testIssueURL, Title: "다른 제목", Body: b}
		}, true, "sealed title and body digest", model.IssueCreateIntentVerificationFailed, testIssueURL, false},
		{"body drift confirmed", func(string) port.IssueProviderIssueCreateCandidate {
			return port.IssueProviderIssueCreateCandidate{URL: testIssueURL, Title: "이슈 제목", Body: "edited"}
		}, true, "sealed title and body digest", model.IssueCreateIntentVerificationFailed, testIssueURL, false},
		{"foreign project confirmed", func(b string) port.IssueProviderIssueCreateCandidate {
			return port.IssueProviderIssueCreateCandidate{URL: "https://github.com/other/repo/issues/2", Title: "이슈 제목", Body: b}
		}, true, "does not match sealed authority", model.IssueCreateIntentInvokedUnknown, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, body := strandedIntent(t)
			finder := &candidateFake{result: port.IssueProviderFindIssueCreateCandidatesResult{Candidates: []port.IssueProviderIssueCreateCandidate{tc.cand(body)}}}
			_, err := newReconciler(h, finder).Reconcile(context.Background(), "io-create", tc.confirm)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			intent := h.intent()
			if intent.Status != tc.status || intent.CanonicalURL != tc.wantURL {
				t.Fatalf("intent = %+v, want status %s url %q", intent, tc.status, tc.wantURL)
			}
			if strings.Count(err.Error(), "does not match sealed authority") > 0 != tc.joinsOut {
				t.Fatalf("err = %v", err)
			}
			if h.store.records["io-create"].IssueURL != "" {
				t.Fatal("a mismatching candidate must never be adopted")
			}
		})
	}
}

func TestIssueReconcileRecordsLiveVerificationFailure(t *testing.T) {
	h, body := strandedIntent(t)
	boom := errors.New("assignee drift")
	h.verifyFn = func(model.IssueOpsRemoteArtifactVerificationRequest) error { return boom }
	finder := &candidateFake{result: port.IssueProviderFindIssueCreateCandidatesResult{Candidates: []port.IssueProviderIssueCreateCandidate{{URL: testIssueURL, Title: "이슈 제목", Body: body}}}}
	_, err := newReconciler(h, finder).Reconcile(context.Background(), "io-create", true)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if intent := h.intent(); intent.Status != model.IssueCreateIntentVerificationFailed || intent.CanonicalURL != testIssueURL || !strings.Contains(intent.Failure, "assignee drift") {
		t.Fatalf("intent = %+v", intent)
	}
}

func TestIssueCreateIntentsRejectOutcomeAndCompletionWithoutIntent(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-bare"}
	store, _ := newArtifactStoreForTest(record)
	intents := NewIssueCreateIntents(store, fixedNow)
	if _, err := intents.Outcome(context.Background(), "io-bare", model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked}); err == nil || !strings.Contains(err.Error(), "no pending issue create intent") {
		t.Fatalf("outcome err = %v", err)
	}
	if _, err := intents.Complete(context.Background(), "io-bare", testIssueURL, "now"); err == nil || !strings.Contains(err.Error(), "no pending issue create intent") {
		t.Fatalf("complete err = %v", err)
	}
	if store.records["io-bare"].UpdatedAt != "" {
		t.Fatal("a rejected transition must not touch the record")
	}
}

// failingUpdateStore fails the Nth Update call and delegates every other call.
type failingUpdateStore struct {
	*artifactStoreForTest
	failAt, updates int
	err             error
}

func (s *failingUpdateStore) Update(ctx context.Context, id string, transition RecordTransition) (model.IssueOpsRecord, error) {
	s.updates++
	if s.updates == s.failAt {
		return model.IssueOpsRecord{}, s.err
	}
	return s.artifactStoreForTest.Update(ctx, id, transition)
}

func TestIssueCreateJoinsOutcomePersistenceFailureWithProviderError(t *testing.T) {
	providerErr, storeErr := errors.New("gh exploded"), errors.New("disk full")
	base, _ := newArtifactStoreForTest(model.IssueOpsRecord{ID: "io-create", Repo: "/repo"})
	// Update 1 seals the intent; update 2 records the provider failure.
	store := &failingUpdateStore{artifactStoreForTest: base, failAt: 2, err: storeErr}
	env := &createEnv{inferred: "github", authority: testAuthority}
	env.invoke = func(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
		return port.IssueProviderCreateIssueResult{}, providerErr
	}
	creator := NewIssueCreator(store, env, NewTemplateBodyResolver(os.ReadFile), NewIssueCreateIntents(store, fixedNow), nil, fixedNow)
	_, err := creator.Create(context.Background(), confirmCmd())
	if !errors.Is(err, providerErr) || !errors.Is(err, storeErr) {
		t.Fatalf("err=%v; want both provider and persistence failures", err)
	}
}

// A pending create whose caller died (no candidate) is released for the same
// create-issue only when the settlement proves nothing was or will be
// created; an unsettled or invoked_unknown intent keeps the refusal.
func TestIssueReconcileReleasesASettledCrashedCreate(t *testing.T) {
	crashed := func(t *testing.T, status string) *createHarness {
		h, _ := strandedIntent(t)
		record := h.store.records["io-create"]
		intent := *record.IssueCreateIntent
		intent.Status, intent.Failure = status, ""
		if status != model.IssueCreateIntentPending {
			intent.Failure = "timeout"
		}
		record.IssueCreateIntent = &intent
		h.store.records["io-create"] = record
		return h
	}
	none := &candidateFake{}
	settled := func(context.Context, model.IssueOpsRecord) (bool, error) { return true, nil }

	h := crashed(t, model.IssueCreateIntentPending)
	preview, err := newReconciler(h, none).WithCrashSettlement(settled).Reconcile(context.Background(), "io-create", false)
	if err != nil || !preview.WouldRelease || preview.NextCommand != "issueops remote reconcile-issue --id io-create --confirm --json" || h.intent().Status != model.IssueCreateIntentPending {
		t.Fatalf("preview=%+v err=%v status=%s", preview, err, h.intent().Status)
	}
	confirmed, err := newReconciler(h, none).WithCrashSettlement(settled).Reconcile(context.Background(), "io-create", true)
	if err != nil || h.intent().Status != model.IssueCreateIntentNotInvoked || confirmed.NextCommand != "issueops next --id io-create --json" {
		t.Fatalf("confirmed=%+v err=%v status=%s", confirmed, err, h.intent().Status)
	}

	// A newer attempt (another reconcile released and create-issue started
	// again) must not be overwritten by a release settled on the old one.
	h = crashed(t, model.IssueCreateIntentPending)
	racing := func(_ context.Context, record model.IssueOpsRecord) (bool, error) {
		newer := h.store.records["io-create"]
		intent := *newer.IssueCreateIntent
		intent.Attempt++
		intent.UpdatedAt = "2026-10-25T00:09:00Z"
		newer.IssueCreateIntent = &intent
		h.store.records["io-create"] = newer
		return true, nil
	}
	if _, err := newReconciler(h, none).WithCrashSettlement(racing).Reconcile(context.Background(), "io-create", true); err == nil || !strings.Contains(err.Error(), "changed since it was settled") || h.intent().Status != model.IssueCreateIntentPending {
		t.Fatalf("racing release: err=%v status=%s", err, h.intent().Status)
	}

	h = crashed(t, model.IssueCreateIntentPending)
	unsettled := func(context.Context, model.IssueOpsRecord) (bool, error) { return false, nil }
	if _, err := newReconciler(h, none).WithCrashSettlement(unsettled).Reconcile(context.Background(), "io-create", true); err == nil || !strings.Contains(err.Error(), "re-run this reconcile") || h.intent().Status != model.IssueCreateIntentPending {
		t.Fatalf("unsettled: err=%v status=%s", err, h.intent().Status)
	}

	h = crashed(t, model.IssueCreateIntentInvokedUnknown)
	if _, err := newReconciler(h, none).WithCrashSettlement(settled).Reconcile(context.Background(), "io-create", true); err == nil || h.intent().Status != model.IssueCreateIntentInvokedUnknown {
		t.Fatalf("invoked_unknown must stay refused: err=%v status=%s", err, h.intent().Status)
	}
}
