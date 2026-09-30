package policy

import "strings"

type PullRequestTargetFacts struct {
	ExpectedBase  string
	RequestedBase string
	HasBaseFlag   bool
}

func DecidePullRequestTarget(facts PullRequestTargetFacts) string {
	if !facts.HasBaseFlag || strings.TrimSpace(facts.RequestedBase) == "" {
		return "pr_target_branch_required"
	}
	if strings.TrimSpace(facts.RequestedBase) != facts.ExpectedBase {
		return "pr_target_branch_mismatch"
	}
	return ""
}
