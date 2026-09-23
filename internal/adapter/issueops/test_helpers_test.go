package issueops

import (
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/contract/issueops"
)

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

// recordIssueOpsProjectDocsReviewForTest는 publication 게이트가 요구하는
// project-doc 반영 판정을 기록한다. 테스트 사이클은 운영 문서를 건드리지
// 않으므로 no-change가 정확한 판정이다.
func recordIssueOpsProjectDocsReviewForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	// no-change는 실제로 읽은 project doc 경로를 요구한다. initIssueOpsRepo가 커밋한
	// CAUTIONS.md를 쓰고, git이 아닌 fixture 디렉터리에만 파일을 만들어 준다.
	root := issueOpsStrictGitRoot(record)
	reviewed := filepath.Join(root, ".issueops", "CAUTIONS.md")
	if _, statErr := os.Stat(reviewed); statErr != nil {
		if err := os.MkdirAll(filepath.Dir(reviewed), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reviewed, []byte("# cautions\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := RecordIssueOpsProjectDocsReviewWithActor(stateRoot, id, IssueOpsProjectDocsReviewRequest{
		Verdict:      "no-change",
		ReviewedDocs: []string{".issueops/CAUTIONS.md"},
		Evidence:     []string{"이 변경은 운영 문서에 남길 결정을 만들지 않는다"},
	}, evidenceRecorderActorForTest(record)); err != nil {
		t.Fatal(err)
	}
}

// recordIssueOpsImplementationReviewForTest는 publication 게이트가 요구하는
// 구현 리뷰를 기록한다. 이 게이트는 execution이 있는 모든 모드에 적용되므로
// direct 픽스처도 pr phase에 들어가기 전에 이 기록이 필요하다.
func recordIssueOpsImplementationReviewForTest(t *testing.T, stateRoot, id string) {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordIssueOpsImplementationReviewWithActor(stateRoot, id, IssueOpsImplementationReviewRequest{
		Verdict:      "pass",
		Findings:     []string{"변경 범위가 이슈 계약을 넘지 않는다"},
		Evidence:     []string{"go test ./internal/adapter/issueops -count=1"},
		ReviewerHost: "claude",
	}, evidenceRecorderActorForTest(record)); err != nil {
		t.Fatal(err)
	}
}

// evidenceRecorderActorForTest는 레코드의 활성 lease holder로 인정되는 actor를
// 만든다. evidence 기록은 owner mutation이라 lease가 있으면 holder만 쓸 수 있다.
// lease holder가 없으면 fence가 적용되지 않으므로 기본 테스트 actor를 쓴다.
func evidenceRecorderActorForTest(record issueops.IssueOpsRecord) IssueOpsActor {
	if record.Execution == nil || record.Execution.Lease.Holder == nil {
		return issueOpsActorForTest(record.WorktreePath)
	}
	holder := record.Execution.Lease.Holder
	actor := IssueOpsActor{Host: holder.Host, SessionID: holder.SessionID, AgentID: holder.AgentID, CWD: record.Execution.Workspace.Root}
	if holder.SessionProcess != nil {
		actor.NativeProcessAncestry = []issueops.NativeProcessReceipt{*holder.SessionProcess}
	}
	return actor
}
