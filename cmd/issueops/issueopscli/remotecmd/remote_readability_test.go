package remotecmd

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/domain/artifactreadability"
	port "issueops/internal/port"
)

// readableIssueBody satisfies the implementation-task contract and the
// readability check.
const readableIssueBody = `## 요약

이슈 본문을 요약이 먼저 오는 짧은 계약으로 바꿉니다. 끝나면 팀원이 요약만 읽고 변경 이유를 압니다.

## 배경

지금 본문은 절이 많아 팀원이 무엇이 바뀌는지 찾기 어렵습니다.

## 완료 기준

- 새로 게시한 이슈의 첫 절이 요약입니다.

## 범위

- 하는 것: 본문 계약 변경
- 하지 않는 것: 이미 게시된 이슈 수정

## 검증

단위 테스트로 필수 절과 요약 규칙을 확인합니다.`

// readablePRBody satisfies the pull-request contract and the readability check.
const readablePRBody = `## 요약

게시 명령이 본문 가독성 검사를 항상 실행하도록 바꿨습니다. 이제 요약이 없는 본문은 게시되지 않습니다.
Closes #1234

## 변경 내용

- 게시 명령이 본문 계약과 가독성 검사를 함께 실행합니다.

## 확인한 것

- 요약이 없는 본문으로 게시를 시도하면 거부되는 것을 명령 테스트로 확인했습니다.

## 리뷰 포인트

- 거부 시점이 원격 호출보다 앞서는지 봐 주세요.`

// readableChildBody satisfies the child-task contract and the readability
// check.
const readableChildBody = `## 요약

부모 이슈 #1234에서 템플릿 렌더러 구현을 맡습니다. 끝나면 렌더러가 새 계약의 필수 절을 출력합니다.

## 완료 기준

- 렌더러 테스트가 필수 절 순서를 확인합니다.

## 범위

- 하는 것: 렌더러 구현
- 하지 않는 것: provider 정책 변경

## 선행 조건과 병합 조건

부모 브랜치에 병합한 뒤 하위 작업을 닫습니다.`

const unreadableBody = `## 변경 내용

요약 절이 없는 본문입니다. 게시 명령은 이 본문을 거부해야 합니다.`

func writeBodyFile(t *testing.T, body string) string {
	t.Helper()
	path := t.TempDir() + "/body.md"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRemoteCreateIssueRefusesCriticalReadability(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordForCreate(t)
	// An empty PATH makes any provider call fail loudly, so a refusal that
	// happened after a provider call could not pass as a readability refusal.
	t.Setenv("PATH", t.TempDir())
	deps := Deps{PrintJSON: func(any) error { return nil }, PrintError: func(error) error { return nil }}

	assertNoIntent := func(t *testing.T) {
		t.Helper()
		stored, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.IssueCreateIntent != nil {
			t.Fatalf("refused create-issue must not seal an issue create intent: %+v", stored.IssueCreateIntent)
		}
	}
	base := []string{"create-issue", "--id", record.ID, "--provider", "github", "--title", "본문 계약 강화", "--label", "enhancement", "--assignee", "octocat", "--confirm"}

	err := Run(append(append([]string{}, base...), "--body-file", writeBodyFile(t, readableIssueBody)), deps)
	if err == nil || !strings.Contains(err.Error(), "--template is required") {
		t.Fatalf("confirm without --template must be refused, got %v", err)
	}
	assertNoIntent(t)

	err = Run(append(append([]string{}, base...), "--template", "implementation_task", "--body-file", writeBodyFile(t, unreadableBody)), deps)
	if err == nil || !strings.Contains(err.Error(), "summary_section_missing") {
		t.Fatalf("confirm with a critical readability finding must be refused, got %v", err)
	}
	assertNoIntent(t)

	var printed []any
	previewDeps := Deps{PrintJSON: func(v any) error { printed = append(printed, v); return nil }}
	if err := Run([]string{"create-issue", "--id", record.ID, "--provider", "github", "--title", "본문 계약 강화", "--body-file", writeBodyFile(t, unreadableBody), "--json"}, previewDeps); err != nil {
		t.Fatalf("preview reports readability instead of failing: %v", err)
	}
	preview, ok := printed[0].(createIssueResponse)
	if !ok {
		t.Fatalf("preview response type = %T", printed[0])
	}
	if preview.Readability.OK || !hasFindingCode(preview.Readability.Critical, "summary_section_missing") {
		t.Fatalf("preview readability = %+v", preview.Readability)
	}
}

func TestRemoteCreateChildAndPRRefuseCriticalReadability(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	path := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	deps := Deps{PrintJSON: func(any) error { return nil }, PrintError: func(error) error { return nil }}
	err := Run([]string{"create-child", "--id", record.ID, "--title", "하위 작업", "--body-file", writeBodyFile(t, unreadableBody), "--label", "bug", "--assignee", "octocat", "--confirm"}, deps)
	if err == nil || !strings.Contains(err.Error(), "summary_section_missing") {
		t.Fatalf("create-child confirm must refuse a critical body before the provider call, got %v", err)
	}

	// create-pr publishes through the handler; its actor check still reads
	// the real process table.
	t.Setenv("PATH", path)
	prRecord := remoteIssueOpsRecord(t)
	ancestry, err := issueopscore.ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	process := ancestry[0]
	handlerCalls := 0
	var printed []any
	prDeps := Deps{
		Publication: PublicationHandlers{Create: func(_ context.Context, _ string, request issueopscore.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			handlerCalls++
			return port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/acme/repo/pull/7", Number: "7"}, nil
		}},
		ObserveProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) {
			return append([]issueopscontract.NativeProcessReceipt(nil), ancestry...), nil
		},
		PrintJSON:  func(v any) error { printed = append(printed, v); return nil },
		PrintError: func(error) error { return nil },
	}
	prArgs := func(bodyFile string) []string {
		return []string{"create-pr", "--id", prRecord.ID, "--provider", "github", "--title", "가독성 검사", "--body-file", bodyFile,
			"--head", prRecord.Branch, "--base", "main", "--label", "bug", "--assignee", "octocat",
			"--host", "claude", "--session-id", "session-513", "--session-pid", strconv.Itoa(process.PID),
			"--session-started-at", process.StartedAt, "--session-executable", process.Executable, "--cwd", prRecord.Repo, "--confirm", "--json"}
	}
	err = Run(prArgs(writeBodyFile(t, unreadableBody)), prDeps)
	if err == nil || !strings.Contains(err.Error(), "summary_section_missing") || handlerCalls != 0 {
		t.Fatalf("create-pr confirm must refuse before publishing: err=%v handlerCalls=%d", err, handlerCalls)
	}
	// The owner packet's create-pr command carries no --template: the single
	// pull_request template applies.
	if err := Run(prArgs(writeBodyFile(t, readablePRBody)), prDeps); err != nil {
		t.Fatalf("create-pr confirm without --template on a readable body: %v", err)
	}
	if handlerCalls != 1 {
		t.Fatalf("handlerCalls = %d, want 1", handlerCalls)
	}
	created, ok := printed[len(printed)-1].(createPRResponse)
	if !ok || !created.Readability.OK || created.URL != "https://github.com/acme/repo/pull/7" {
		t.Fatalf("create-pr response = %#v", printed[len(printed)-1])
	}
}

func hasFindingCode(findings []artifactreadability.Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestRemoteSyncRefusesCriticalAndReportsLiveReadability(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	t.Setenv("PATH", t.TempDir())
	deps := Deps{
		ObserveProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) {
			return []issueopscontract.NativeProcessReceipt{{PID: os.Getpid(), StartedAt: "2026-09-24T00:00:00Z", Executable: "go"}}, nil
		},
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
	}
	err := Run([]string{"sync-issue", "--id", record.ID, "--provider", "github", "--body-file", writeBodyFile(t, unreadableBody),
		"--expected-body-sha256", strings.Repeat("a", 64), "--confirm", "--json"}, deps)
	if err == nil || !strings.Contains(err.Error(), "summary_section_missing") {
		t.Fatalf("sync-issue confirm must refuse a critical body before reading the remote, got %v", err)
	}
	err = Run([]string{"sync-issue", "--id", record.ID, "--provider", "github", "--template", "child_task", "--body-file", writeBodyFile(t, readableIssueBody), "--json"}, deps)
	if err == nil || !strings.Contains(err.Error(), "does not apply") {
		t.Fatalf("sync-issue must pass --template through and refuse a mismatched one, got %v", err)
	}
}

func TestReflectCompletionRequiresReadableResult(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	record.RemoteArtifact = &issueopscontract.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/7"}
	if _, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	verified := 0
	deps := Deps{
		VerifyMerged: func(issueopscontract.IssueOpsRemoteArtifactVerification) error { verified++; return nil },
		PrintJSON:    func(any) error { return nil },
		PrintError:   func(error) error { return nil },
	}
	err := Run([]string{"reflect-completion", "--id", record.ID, "--provider", "github", "--confirm", "--json"}, deps)
	if err == nil || !strings.Contains(err.Error(), "--body-file is required") || verified != 0 {
		t.Fatalf("confirm without a draft must fail before the merge readback: err=%v verified=%d", err, verified)
	}
	draft := "두 이슈를 서로 다른 세션에서 동시에 진행해도 간섭하지 않음을 확인했습니다.\n\n- 커밋: " + strings.Repeat("ab", 20) + "\n"
	err = Run([]string{"reflect-completion", "--id", record.ID, "--provider", "github", "--body-file", writeBodyFile(t, draft), "--confirm", "--json"}, deps)
	if err == nil || !strings.Contains(err.Error(), "commit_sha_full") {
		t.Fatalf("a draft with a full commit SHA must be refused, got %v", err)
	}
}
