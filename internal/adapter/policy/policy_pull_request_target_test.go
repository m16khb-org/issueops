package policy

import (
	"testing"

	policydomain "issueops/internal/contract/policy"
)

func preparedBaseBranch(base string, found bool) PreparedBaseBranchLookup {
	return func(string) (string, bool) { return base, found }
}

func TestPullRequestTargetDenyBlocksMismatchedAndMissingTarget(t *testing.T) {
	tests := []struct {
		name     string
		argv     []string
		reason   string
		expected string
	}{
		{
			name:     "mismatched target hits the release branch instead of the parent",
			argv:     []string{"glab", "mr", "create", "--target-branch", "release/stg"},
			reason:   "pr_target_branch_mismatch",
			expected: "parent/umbrella-work",
		},
		{
			name:     "joined mismatched target",
			argv:     []string{"glab", "mr", "create", "--target-branch=main"},
			reason:   "pr_target_branch_mismatch",
			expected: "parent/umbrella-work",
		},
		{
			name:     "github mismatched base",
			argv:     []string{"gh", "pr", "create", "--base", "main", "--title", "t"},
			reason:   "pr_target_branch_mismatch",
			expected: "parent/umbrella-work",
		},
		{
			name:     "no target flag at all",
			argv:     []string{"glab", "mr", "create", "--fill"},
			reason:   "pr_target_branch_required",
			expected: "parent/umbrella-work",
		},
		{
			name:     "flag present but empty",
			argv:     []string{"glab", "mr", "create", "--target-branch="},
			reason:   "pr_target_branch_required",
			expected: "parent/umbrella-work",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason, expected := pullRequestTargetDeny("/repo", "/repo/worktree", tc.argv, preparedBaseBranch("parent/umbrella-work", true))
			if reason != tc.reason {
				t.Fatalf("reason = %q, want %q", reason, tc.reason)
			}
			if expected != tc.expected {
				t.Fatalf("expected branch = %q, want %q", expected, tc.expected)
			}
		})
	}
}

func TestPullRequestTargetDenyAllowsMatchingTarget(t *testing.T) {
	argv := []string{"glab", "mr", "create", "--target-branch", "parent/umbrella-work"}
	if reason, _ := pullRequestTargetDeny("/repo", "/repo/worktree", argv, preparedBaseBranch("parent/umbrella-work", true)); reason != "" {
		t.Fatalf("reason = %q, want empty", reason)
	}
}

// 진행 중인 사이클이 없으면 판정하지 않는다. IssueOps 밖에서 여는 일상적인
// PR/MR까지 막으면 가드가 아니라 방해가 된다.
func TestPullRequestTargetDenySkipsWithoutActiveCycle(t *testing.T) {
	argv := []string{"glab", "mr", "create", "--target-branch", "release/stg"}
	if reason, _ := pullRequestTargetDeny("/repo", "/repo/worktree", argv, preparedBaseBranch("", false)); reason != "" {
		t.Fatalf("reason = %q, want empty", reason)
	}
}

func TestPullRequestTargetDenyIgnoresNonCreateCommands(t *testing.T) {
	for _, argv := range [][]string{
		{"glab", "mr", "view", "1"},
		{"gh", "pr", "merge", "494"},
		{"git", "push"},
	} {
		if reason, _ := pullRequestTargetDeny("/repo", "/repo/worktree", argv, preparedBaseBranch("parent/umbrella-work", true)); reason != "" {
			t.Fatalf("argv %v reason = %q, want empty", argv, reason)
		}
	}
}

// cwd가 비면 workspace root로 조회한다. 원격 쓰기 명령이 소스 체크아웃에서
// 실행되는 경우를 위한 폴백이다.
func TestPullRequestTargetDenyFallsBackToWorkspaceRoot(t *testing.T) {
	var seen string
	lookup := func(path string) (string, bool) {
		seen = path
		return "parent/umbrella-work", true
	}

	argv := []string{"glab", "mr", "create", "--target-branch", "release/stg"}
	if reason, _ := pullRequestTargetDeny("/repo", "", argv, lookup); reason != "pr_target_branch_mismatch" {
		t.Fatalf("reason = %q, want pr_target_branch_mismatch", reason)
	}
	if seen != "/repo" {
		t.Fatalf("lookup path = %q, want /repo", seen)
	}
}

// EvaluateCommandPolicy를 통과시켜, 호출부가 지워지면 실패하게 한다.
// pullRequestTargetDeny 단위 테스트만 있으면 평가 본문에서 호출을 빼도 그대로
// 초록이라, 2026-08-27과 같은 방식으로 가드가 조용히 사라질 수 있다.
func TestEvaluateCommandPolicyDeniesMistargetedPullRequest(t *testing.T) {
	root := t.TempDir()

	result := NewEvaluator(preparedBaseBranch("parent/umbrella-work", true)).Evaluate(policydomain.CommandPolicyRequest{
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

	result := NewEvaluator(preparedBaseBranch("parent/umbrella-work", true)).Evaluate(policydomain.CommandPolicyRequest{
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
	if result := one.Evaluate(request); containsString(result.DenyReasons, "pr_target_branch_mismatch") {
		t.Fatalf("first evaluator denied its own prepared base: %v", result.DenyReasons)
	}
	if result := two.Evaluate(request); !containsString(result.DenyReasons, "pr_target_branch_mismatch") {
		t.Fatalf("second evaluator accepted another prepared base: %v", result.DenyReasons)
	}
}
