package riskqa

import "testing"

func TestPlanPreservesGitWarningsAlongsideDomainTier(t *testing.T) {
	service := Service{ChangedPaths: func(string) ([]string, []string) {
		return []string{"internal/policy/rule.go"}, []string{"git warning"}
	}}
	plan := service.Plan("/repo")
	if plan.Tier != "elevated" || len(plan.Commands) != 2 || len(plan.Reasons) == 0 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}
