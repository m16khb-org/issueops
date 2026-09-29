package policy

import (
	"testing"
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
