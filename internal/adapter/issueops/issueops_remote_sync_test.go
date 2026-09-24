package issueops

import (
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

// 관련 이슈 댓글은 팀원이 읽는다. 한국어 제목과 링크 종류를 쓰고, 사이클 ID 같은
// 하네스 값을 싣지 않는다(#513).
func TestIssueGraphCommentIsKorean(t *testing.T) {
	record := issueops.IssueOpsRecord{
		ID: "io-0123456789ab",
		IssueLinks: []issueops.IssueOpsIssueLink{
			{Type: "depends-on", URL: "https://github.com/acme/repo/issues/2", Title: "선행 작업"},
			{Type: "child", URL: "https://github.com/acme/repo/issues/3"},
		},
	}
	got := renderIssueGraphComment(record)
	for _, want := range []string{"## 관련 이슈\n", "- **선행 이슈**: https://github.com/acme/repo/issues/2 (선행 작업)", "- **하위 작업**: https://github.com/acme/repo/issues/3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("comment missing %q:\n%s", want, got)
		}
	}
	for _, leaked := range []string{"Related Issue Graph", "Cycle:", "io-0123456789ab", "issueops IssueOps"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("comment must not carry %q:\n%s", leaked, got)
		}
	}
	empty, err := SyncRemoteIssueGraph(issueops.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/1"})
	if err != nil || empty["synced"] != false {
		t.Fatalf("no links means no comment: %v %v", empty, err)
	}
}
