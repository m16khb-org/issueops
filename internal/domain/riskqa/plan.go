// Package riskqa derives the risk-based QA tier and verification commands from
// the set of changed paths. It is pure policy in the domain layer.
package riskqa

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"

	riskqacontract "issueops/internal/contract/riskqa"
)

// FullRaceCommand is the full-repository race test command that the elevated
// tier requires.
const FullRaceCommand = "go test -race ./... -count=1"

// CoversFullGoTest reports whether plan includes FullRaceCommand and its scope
// carries no error.
func CoversFullGoTest(plan riskqacontract.TierPlan) bool {
	return (plan.Scope == nil || plan.Scope.Error == "") && slices.Contains(plan.Commands, FullRaceCommand)
}

// PlanFromPaths picks the QA tier for paths: "standard" without local changes,
// "static" for Go changes, and "elevated" when Go changes touch policy, MCP,
// adapter, state, or other harness surfaces. Reasons are sorted.
func PlanFromPaths(paths []string) riskqacontract.TierPlan {
	plan := riskqacontract.TierPlan{Tier: "standard", ChangedPaths: riskqacontract.NormalizePaths(paths), Reasons: []string{}, Commands: []string{}}
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
		plan.Reasons = append(plan.Reasons, "go changes touch policy, MCP, adapter, state, or harness orchestration surfaces")
		plan.Commands = append([]string{FullRaceCommand}, plan.Commands...)
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
	for _, token := range []string{"worker", "policy", "state", "mcp", "adapter", "install", "hook", "self_augment", "self-augment"} {
		if strings.Contains(path, token) {
			return true
		}
	}
	return false
}
