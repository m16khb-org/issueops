package policy

import "testing"

func TestDecidePullRequestTargetRequiresPreparedBase(t *testing.T) {
	for _, test := range []struct {
		facts  PullRequestTargetFacts
		reason string
	}{
		{PullRequestTargetFacts{ExpectedBase: "work", HasBaseFlag: false}, "pr_target_branch_required"},
		{PullRequestTargetFacts{ExpectedBase: "work", HasBaseFlag: true, RequestedBase: ""}, "pr_target_branch_required"},
		{PullRequestTargetFacts{ExpectedBase: "work", HasBaseFlag: true, RequestedBase: "main"}, "pr_target_branch_mismatch"},
		{PullRequestTargetFacts{ExpectedBase: "work", HasBaseFlag: true, RequestedBase: "work"}, ""},
	} {
		if got := DecidePullRequestTarget(test.facts); got != test.reason {
			t.Fatalf("facts=%+v: reason=%q, want %q", test.facts, got, test.reason)
		}
	}
}
