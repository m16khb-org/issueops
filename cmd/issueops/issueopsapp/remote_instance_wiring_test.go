package issueopsapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/issueopscli"
	adapter "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestRemoteCLIInstancesKeepCapturedState(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	repo := makeGitRepoForContract(t)
	var deps [2]issueopscli.Dependencies
	var records [2]model.IssueOpsRecord
	for i := range deps {
		t.Setenv("ISSUEOPS_STATE_DIR", roots[i])
		var err error
		records[i], err = startIssueOpsFixture(issueOpsStateRoot(), model.IssueOpsStartRequest{Repo: repo, Branch: "991-remote-state"})
		if err != nil {
			t.Fatal(err)
		}
		deps[i] = issueOpsCLIDependencies()
	}
	t.Setenv("ISSUEOPS_STATE_DIR", roots[2])
	t.Setenv("PATH", t.TempDir())
	for _, i := range []int{0, 1, 0} {
		raw := captureStdoutForContract(t, func() error {
			return issueopscli.RunIssueOpsWithDependencies([]string{"remote", "create-issue", "--id", records[i].ID, "--provider", "github", "--title", "Preview", "--body", "Body", "--label", "bug", "--assignee", "test", "--json"}, deps[i])
		})
		var result port.IssueProviderCreateIssueResult
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatal(err)
		}
		if result.URL != "" || result.Preview == "" || result.Provider != "github" {
			t.Fatalf("instance %d preview: %+v", i, result)
		}
	}
}

func TestRemoteChildCreationUsesRootApplicationAndPersistsLink(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := issueOpsStateRoot()
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: makeGitRepoForContract(t), Branch: "1234-child-root"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = newIssueLinker(root).Issue(context.Background(), record.ID, "https://github.com/acme/repo/issues/1234", nil)
	if err != nil {
		t.Fatal(err)
	}
	record, err = newBranchPreparer(root).Prepare(context.Background(), record.ID, model.IssueOpsBranchPrepareRequest{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main", LinkVerified: true}, nil)
	if err != nil {
		t.Fatal(err)
	}

	bin := t.TempDir()
	script := `#!/bin/sh
if [ "$1 $2 $3" = "issue create --help" ]; then printf "  --title string\n";exit 0;fi
if [ "$1 $2" = "issue create" ]; then
 printf 'https://github.com/acme/repo/issues/34\n';exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/34" ]; then
 printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","labels":[{"name":"bug"}],"assignees":[{"login":"octocat"}]}';exit 0
fi
if [ "$1 $2" = "api -X" ] && [ "$3" = "POST" ]; then
 printf '{"ok":true}';exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/1234/sub_issues" ]; then
 printf '[{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34"}]';exit 0
fi
exit 2
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	deps := issueOpsCLIDependencies()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	raw := captureStdoutForContract(t, func() error {
		return issueopscli.RunIssueOpsWithDependencies([]string{"remote", "create-child", "--id", record.ID, "--title", "Child", "--body", readableWiringChildBody, "--label", "bug", "--assignee", "octocat", "--confirm", "--json"}, deps)
	})
	var result port.IssueProviderCreateChildResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if !result.HierarchyVerified || result.ChildURL != "https://github.com/acme/repo/issues/34" {
		t.Fatalf("result=%+v", result)
	}
	stored, err := adapter.ReadIssueOps(root, record.ID)
	if err != nil || len(stored.IssueLinks) != 1 || stored.IssueLinks[0].URL != result.ChildURL {
		t.Fatalf("persisted links=%+v err=%v", stored.IssueLinks, err)
	}
}

const readableWiringChildBody = `## 요약

부모 이슈 #1234에서 템플릿 렌더러 구현을 맡습니다. 끝나면 렌더러가 새 계약의 필수 절을 출력합니다.

## 완료 기준

- 렌더러 테스트가 필수 절 순서를 확인합니다.

## 범위

- 하는 것: 렌더러 구현
- 하지 않는 것: provider 정책 변경

## 선행 조건과 병합 조건

부모 브랜치에 병합한 뒤 하위 작업을 닫습니다.`
