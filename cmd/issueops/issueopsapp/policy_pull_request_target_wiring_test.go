package issueopsapp

import (
	"testing"

	policycli "issueops/cmd/issueops/policycli"
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
	original := policycli.EvaluateCommandPolicy
	t.Cleanup(func() { policycli.EvaluateCommandPolicy = original })
	root := t.TempDir()
	lookupPath := ""
	configurePolicyAndGitObserversWithLookup(func(path string) (string, bool) {
		lookupPath = path
		return "parent/umbrella-work", true
	})

	if policycli.EvaluateCommandPolicy == nil {
		t.Fatal("composition root가 policy PR/MR target lookup을 설치하지 않았다; " +
			"설치가 빠지면 잘못된 타겟의 PR/MR이 사전 거부 없이 열린다")
	}
	result := policycli.EvaluateCommandPolicy(policycontract.CommandPolicyRequest{
		WorkspaceRoot: root, CWD: root,
		Argv:    []string{"glab", "mr", "create", "--target-branch", "release/stg"},
		Timeout: "30s", WriteAllowed: true, NetworkAllowed: true,
	})
	if lookupPath != root || !containsString(result.DenyReasons, "pr_target_branch_mismatch") {
		t.Fatalf("lookup path = %q, deny reasons = %v", lookupPath, result.DenyReasons)
	}
}
