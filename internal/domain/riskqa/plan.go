package riskqa

import (
	"path/filepath"
	"sort"
	"strings"

	riskqacontract "issueops/internal/contract/riskqa"
)

func PlanFromPaths(paths []string) riskqacontract.RiskQATierPlan {
	plan := riskqacontract.RiskQATierPlan{Tier: "standard", ChangedPaths: riskqacontract.NormalizePaths(paths), Reasons: []string{}, Commands: []string{}}
	if len(plan.ChangedPaths) == 0 {
		plan.Reasons = append(plan.Reasons, "working tree has no local changes")
		return plan
	}
	goChanged := false
	sensitive := false
	for _, path := range plan.ChangedPaths {
		if strings.HasSuffix(path, ".go") {
			goChanged = true
		}
		if isRiskSensitivePath(path) {
			sensitive = true
		}
	}
	if goChanged {
		plan.Tier = "static"
		plan.Reasons = append(plan.Reasons, "go changes detected")
		plan.Commands = append(plan.Commands, "go vet ./...")
	}
	if goChanged && sensitive {
		plan.Tier = "elevated"
		plan.Reasons = append(plan.Reasons, "go changes touch policy, MCP, adapter, daemon, state, or harness orchestration surfaces")
		plan.Commands = append([]string{"go test -race ./... -count=1"}, plan.Commands...)
	}
	if !goChanged {
		plan.Reasons = append(plan.Reasons, "no Go changes detected; race/static tier skipped")
	}
	sort.Strings(plan.Reasons)
	return plan
}

func isRiskSensitivePath(path string) bool {
	path = filepath.ToSlash(path)
	if strings.HasPrefix(path, "cmd/issueops/") || strings.HasPrefix(path, "internal/") {
		return true
	}
	for _, token := range []string{"daemon", "worker", "policy", "state", "mcp", "adapter", "install", "hook", "self_augment", "self-augment"} {
		if strings.Contains(path, token) {
			return true
		}
	}
	return false
}
