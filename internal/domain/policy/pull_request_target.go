package policy

import "strings"

// PullRequestTargetFacts holds what a pull-request creation command asks for: the
// base branch the cycle expects, the base branch the command requested, and
// whether a base flag was present at all.
type PullRequestTargetFacts struct {
	ExpectedBase  string
	RequestedBase string
	HasBaseFlag   bool
}

// DecidePullRequestTarget returns the deny reason for a pull-request base branch,
// or an empty string when the requested base equals the expected one.
// A missing or blank base yields pr_target_branch_required and a different base
// yields pr_target_branch_mismatch.
func DecidePullRequestTarget(facts PullRequestTargetFacts) string {
	if !facts.HasBaseFlag || strings.TrimSpace(facts.RequestedBase) == "" {
		return "pr_target_branch_required"
	}
	if strings.TrimSpace(facts.RequestedBase) != facts.ExpectedBase {
		return "pr_target_branch_mismatch"
	}
	return ""
}
