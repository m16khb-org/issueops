package commandparse

import "testing"

func TestParseRemotePullRequestCreateMinimalAndTrailingBaseFlag(t *testing.T) {
	parsed, ok := ParseRemotePullRequestCreate([]string{"gh", "pr", "create"})
	if !ok || parsed.Provider != "github" || parsed.Kind != "pr" || parsed.HasBaseFlag || parsed.BaseBranch != "" {
		t.Fatalf("bare gh pr create = %+v, ok=%v", parsed, ok)
	}
	for _, flag := range []string{"--base", "-B", "--target-branch"} {
		parsed, ok := ParseRemotePullRequestCreate([]string{"gh", "pr", "create", flag})
		if !ok || !parsed.HasBaseFlag || parsed.BaseBranch != "" {
			t.Fatalf("trailing %s = %+v, ok=%v", flag, parsed, ok)
		}
	}
}
