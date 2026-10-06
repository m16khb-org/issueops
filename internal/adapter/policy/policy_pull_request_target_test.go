package policy

import (
	"testing"

	policydomain "issueops/internal/contract/policy"
)

func preparedBaseBranch(base string, found bool) func(string) (string, bool) {
	return func(string) (string, bool) { return base, found }
}

// 정책 평가 전체를 통과시켜, 호출부가 지워지면 실패하게 한다.
// pullRequestTargetDeny 단위 테스트만 있으면 평가 본문에서 호출을 빼도 그대로
// 초록이라, 2026-08-27과 같은 방식으로 가드가 조용히 사라질 수 있다.
func TestEvaluateCommandPolicyDeniesMistargetedPullRequest(t *testing.T) {
	root := t.TempDir()

	result := NewEvaluator(preparedBaseBranch("parent/umbrella-work", true)).service().Evaluate(policydomain.CommandPolicyRequest{
		WorkspaceRoot:  root,
		CWD:            root,
		Argv:           []string{"glab", "mr", "create", "--target-branch", "release/stg"},
		Timeout:        "30s",
		WriteAllowed:   true,
		NetworkAllowed: true,
	})

	if result.Allowed {
		t.Fatalf("mistargeted MR create was allowed: %+v", result)
	}
	if !containsString(result.DenyReasons, "pr_target_branch_mismatch") {
		t.Fatalf("DenyReasons = %v, want pr_target_branch_mismatch", result.DenyReasons)
	}
	if !containsString(result.Warnings, "pr_target_branch_expected=parent/umbrella-work") {
		t.Fatalf("Warnings = %v, want the expected branch", result.Warnings)
	}
}

func TestEvaluateCommandPolicyAllowsCorrectlyTargetedPullRequest(t *testing.T) {
	root := t.TempDir()

	result := NewEvaluator(preparedBaseBranch("parent/umbrella-work", true)).service().Evaluate(policydomain.CommandPolicyRequest{
		WorkspaceRoot:  root,
		CWD:            root,
		Argv:           []string{"glab", "mr", "create", "--target-branch", "parent/umbrella-work"},
		Timeout:        "30s",
		WriteAllowed:   true,
		NetworkAllowed: true,
	})

	for _, reason := range result.DenyReasons {
		if reason == "pr_target_branch_mismatch" || reason == "pr_target_branch_required" {
			t.Fatalf("correctly targeted MR create denied: %+v", result.DenyReasons)
		}
	}
}

func TestPolicyEvaluatorsKeepPreparedBasesIsolated(t *testing.T) {
	root := t.TempDir()
	request := policydomain.CommandPolicyRequest{
		WorkspaceRoot: root, CWD: root,
		Argv:    []string{"glab", "mr", "create", "--target-branch", "parent/one"},
		Timeout: "30s", WriteAllowed: true, NetworkAllowed: true,
	}
	one := NewEvaluator(preparedBaseBranch("parent/one", true))
	two := NewEvaluator(preparedBaseBranch("parent/two", true))
	if result := one.service().Evaluate(request); containsString(result.DenyReasons, "pr_target_branch_mismatch") {
		t.Fatalf("first evaluator denied its own prepared base: %v", result.DenyReasons)
	}
	if result := two.service().Evaluate(request); !containsString(result.DenyReasons, "pr_target_branch_mismatch") {
		t.Fatalf("second evaluator accepted another prepared base: %v", result.DenyReasons)
	}
}
