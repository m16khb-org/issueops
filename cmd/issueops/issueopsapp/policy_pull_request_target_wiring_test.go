package issueopsapp

import (
	"testing"

	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	policycontract "issueops/internal/contract/policy"
)

// TestPolicyPullRequestTargetLookupIsWired는 PR/MR target 가드가 composition
// root에 실제로 설치되는지 본다.
//
// 이 테스트가 없으면 배선 한 줄이 사라져도 policy 패키지의 단위 테스트와
// 통합 테스트는 모두 통과한다. 그 테스트들은 lookup을 직접 주입하기
// 때문이다. 2026-08-27에 legacy hook 표면을 지우면서 배선 파일
// remoteartifact_wiring.go가 함께 사라졌고, 가드 구현과 그 테스트는 그대로
// 남아 CI가 매번 초록을 보고했다. 다음 날 자식 Task MR이 부모 작업 브랜치가
// 아니라 release/stg를 타겟해 열렸다.
//
// 여기서 composition root가 주입한 조회 함수까지 실제 판정에 쓰이는지 확인한다.
func TestPolicyPullRequestTargetLookupIsWired(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := makeGitRepoForContract(t)
	record, err := startIssueOpsFixture(issueOpsStateRoot(), model.IssueOpsStartRequest{Repo: root, Branch: "79-child"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/79"
	record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "parent/umbrella-work", LinkVerified: true, CreatedAt: record.CreatedAt}
	if _, err = core.WriteIssueOps(issueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	service := newPolicyService()
	lookupPath := ""
	lookup := service.PreparedBaseBranch
	if lookup == nil {
		t.Fatal("composition root did not wire prepared branch reader")
	}
	service.PreparedBaseBranch = func(path string) (string, bool) { lookupPath = path; return lookup(path) }
	result := service.Evaluate(policycontract.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: []string{"glab", "mr", "create", "--target-branch", "release/stg"}, Timeout: "30s", WriteAllowed: true, NetworkAllowed: true})
	if lookupPath != root || !containsString(result.DenyReasons, "pr_target_branch_mismatch") {
		t.Fatalf("lookup path = %q, deny reasons = %v", lookupPath, result.DenyReasons)
	}
}

func TestPolicyLookupReadsCurrentStateRootOnEachEvaluation(t *testing.T) {
	repo := makeGitRepoForContract(t)
	for _, base := range []string{"78-first-parent", "80-second-parent"} {
		t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
		root := issueOpsStateRoot()
		record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "79-child"})
		if err != nil {
			t.Fatal(err)
		}
		record.IssueURL = "https://github.com/acme/repo/issues/79"
		record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: base, LinkVerified: true, CreatedAt: record.CreatedAt}
		if _, err = core.WriteIssueOps(root, record); err != nil {
			t.Fatal(err)
		}
		service := newPolicyService()
		request := policycontract.CommandPolicyRequest{WorkspaceRoot: repo, CWD: repo, Argv: []string{"glab", "mr", "create", "--target-branch", "main"}, Timeout: "30s", WriteAllowed: true, NetworkAllowed: true}
		if got := service.Evaluate(request); !containsString(got.DenyReasons, "pr_target_branch_mismatch") {
			t.Fatalf("current stored parent ignored: %+v", got.DenyReasons)
		}
		request.Argv[len(request.Argv)-1] = base
		if got := service.Evaluate(request); containsString(got.DenyReasons, "pr_target_branch_mismatch") {
			t.Fatalf("current stored parent refused: %+v", got.DenyReasons)
		}
	}
}
