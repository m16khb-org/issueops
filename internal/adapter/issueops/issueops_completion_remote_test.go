package issueops

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type fakeCompletionProvider struct {
	updateReq  *port.IssueProviderUpdateIssueBodySectionRequest
	updateRes  port.IssueProviderUpdateIssueBodySectionResult
	closeReq   *port.IssueProviderCloseIssueRequest
	closeRes   port.IssueProviderCloseIssueResult
	updateErr  error
	closeError error
}

func (p *fakeCompletionProvider) Name() string { return "github" }
func (p *fakeCompletionProvider) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}
func (p *fakeCompletionProvider) CreatePullRequest(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return port.IssueProviderCreatePullRequestResult{}, nil
}
func (p *fakeCompletionProvider) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}
func (p *fakeCompletionProvider) CloseChild(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return port.IssueProviderCloseChildResult{}, nil
}
func (p *fakeCompletionProvider) CloseIssue(req port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	p.closeReq = &req
	return p.closeRes, p.closeError
}
func (p *fakeCompletionProvider) UpdateIssueBodySection(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	p.updateReq = &req
	return p.updateRes, p.updateErr
}

func completionTestRecord(t *testing.T) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "81-completion"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/81"
	record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/85",
	}
	if err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		_, e := writeIssueOps(stateRoot, record)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record
}

func TestReflectIssueCompletionGates(t *testing.T) {
	stateRoot, record := completionTestRecord(t)
	prov := &fakeCompletionProvider{}

	if _, _, _, err := ReflectIssueCompletion(stateRoot, record.ID, readableResult, false, true, prov); err == nil {
		t.Fatal("missing merge evidence must be rejected")
	}
	if prov.updateReq != nil {
		t.Fatal("provider must not be called without merge evidence")
	}

	prov.updateRes = port.IssueProviderUpdateIssueBodySectionResult{OK: true, Preview: "[dry-run]"}
	got, result, _, err := ReflectIssueCompletion(stateRoot, record.ID, readableResult, true, false, prov)
	if err != nil || result.Preview == "" {
		t.Fatalf("preview must pass through: %v %+v", err, result)
	}
	if got.RemoteCompletion != nil {
		t.Fatal("preview must not stamp the local completion cache")
	}
	if prov.updateReq.Section != port.IssueBodySectionCompletion || prov.updateReq.Completion == nil {
		t.Fatalf("completion payload must be routed: %+v", prov.updateReq)
	}

	prov.updateRes = port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: true, URL: record.IssueURL}
	got, _, _, err = ReflectIssueCompletion(stateRoot, record.ID, readableResult, true, true, prov)
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteCompletion == nil || got.RemoteCompletion.ReflectedAt == "" {
		t.Fatalf("confirmed update must stamp ReflectedAt: %+v", got.RemoteCompletion)
	}
}

// readableResult is a progress-report draft that passes the completion check.
const readableResult = "두 이슈를 서로 다른 세션에서 동시에 진행해도 간섭하지 않음을 실제 실행으로 확인했습니다.\n\n" +
	"- 계획: 한 사이클을 끝까지 실행한다.\n- 구현: 보고서를 작성했다(PR #85)."

// 진행 결과는 사람이 쓴 원고로만 반영한다. 원고가 없거나, 커밋 SHA 전문이나
// 로컬 경로가 있거나, 2,000자를 넘으면 provider를 부르기 전에 거부한다(#513).
func TestReflectCompletionRequiresReadableResult(t *testing.T) {
	stateRoot, record := completionTestRecord(t)
	prov := &fakeCompletionProvider{updateRes: port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: true, URL: record.IssueURL}}
	for _, tc := range []struct {
		name, body, want string
	}{
		{"missing", "", "--body-file"},
		{"harness values", readableResult + "\n- 커밋: " + strings.Repeat("ab", 20) + "\n- 작업 공간: /Users/dev/wt", "commit_sha_full"},
		{"too long", strings.Repeat("완료 보고 문장입니다. ", 250), "result_too_long"},
	} {
		_, _, _, err := ReflectIssueCompletion(stateRoot, record.ID, tc.body, true, true, prov)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error = %v, want mention of %q", tc.name, err, tc.want)
		}
		if tc.name == "harness values" && !strings.Contains(err.Error(), "local_path") {
			t.Fatalf("local paths are critical in a progress report: %v", err)
		}
		if prov.updateReq != nil {
			t.Fatalf("%s: a refused draft must not reach the provider", tc.name)
		}
	}

	_, _, report, err := ReflectIssueCompletion(stateRoot, record.ID, readableResult, true, true, prov)
	if err != nil || !report.OK {
		t.Fatalf("a readable draft must be reflected: err=%v report=%+v", err, report)
	}
	c := prov.updateReq.Completion
	if c == nil || c.ResultBody != readableResult || c.RemoteArtifactURL != "https://github.com/acme/repo/pull/85" {
		t.Fatalf("the payload carries the draft and the PR URL only: %+v", c)
	}
}

func TestCloseIssueOpsRemoteIssueGatesAndStamps(t *testing.T) {
	stateRoot, record := completionTestRecord(t)
	prov := &fakeCompletionProvider{}

	if _, _, err := CloseIssueOpsRemoteIssue(stateRoot, record.ID, false, true, prov); err == nil {
		t.Fatal("missing merge evidence must be rejected")
	}

	prov.closeRes = port.IssueProviderCloseIssueResult{OK: true, Preview: "[dry-run]"}
	got, result, err := CloseIssueOpsRemoteIssue(stateRoot, record.ID, true, false, prov)
	if err != nil || result.Preview == "" {
		t.Fatalf("preview must pass through: %v %+v", err, result)
	}
	if got.RemoteCompletion != nil && got.RemoteCompletion.IssueClosedAt != "" {
		t.Fatal("preview must not stamp the close cache")
	}

	prov.closeRes = port.IssueProviderCloseIssueResult{OK: true, Closed: true, IssueURL: record.IssueURL}
	got, _, err = CloseIssueOpsRemoteIssue(stateRoot, record.ID, true, true, prov)
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteCompletion == nil || got.RemoteCompletion.IssueClosedAt == "" {
		t.Fatalf("verified close must stamp IssueClosedAt: %+v", got.RemoteCompletion)
	}
	if prov.closeReq.IssueURL != record.IssueURL {
		t.Fatalf("close must target the linked issue: %+v", prov.closeReq)
	}
}
