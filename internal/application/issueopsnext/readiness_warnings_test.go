package issueopsnext

import (
	"context"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

// readiness가 이미 내던 경고를 next가 버리면 base drift 같은 관측이 사용자에게
// 닿지 않는다. 차단하지 않는 관측도 그대로 전달한다.
func TestNextPassesLocalReadinessWarningsThrough(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID: "io-warn", Repo: "/repo", Branch: "9-warn",
		Phase: issueopscontract.IssueOpsPhaseAISlopClean,
	}
	entries := []issueopsinventorycontract.ListEntry{{ID: record.ID, Phase: record.Phase, Branch: record.Branch}}
	ports := testPorts(entries, map[string]issueopscontract.IssueOpsRecord{record.ID: record})
	ports.LocalReadiness = func(issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
		return issueopscontract.IssueOpsReadiness{
			OK: true, Ready: false, Missing: []string{"worktree_clean"},
			Warnings: []string{"base_advanced: origin/main is not an ancestor of HEAD; run issueops execution sync-base --id io-warn --preview"},
		}
	}
	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, warning := range result.Warnings {
		if strings.HasPrefix(warning, "base_advanced:") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a readiness warning must reach next: %v", result.Warnings)
	}
	// 이 record는 실제 stage로 분류되어 early return하는 경로를 탄다. 경고가
	// 특정 분기에만 있으면 그 경로에서 사라지므로, stage가 unknown이 아님을 함께
	// 확인해 passthrough가 공통 경로에 있음을 증명한다.
	if result.Stage.Key == "unknown" || result.Stage.Key == "" {
		t.Fatalf("the fixture must classify into a real stage, got %q", result.Stage.Key)
	}
}
