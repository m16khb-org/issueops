package riskqa

import (
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/riskqa"
	selfverify "issueops/internal/contract/selfverify"
)

func TestExecuteStopsAtFirstFailedRiskCommand(t *testing.T) {
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	var called []string
	result := Execute("/repo", ExecuteDeps{
		Plan: func(string) contract.RiskQATierPlan {
			return contract.RiskQATierPlan{Tier: "elevated", Commands: []string{"go test -race ./... -count=1", "go vet ./..."}}
		},
		Run: func(_ string, command string) selfverify.StepResult {
			called = append(called, command)
			if command != "go test -race ./... -count=1" {
				return selfverify.StepResult{Label: command, OK: false, Error: "unexpected command"}
			}
			return selfverify.StepResult{Label: "risk QA race test", Command: command, OK: false, Error: "race failed"}
		},
		RenderPlan: func(contract.RiskQATierPlan) string { return `{"tier":"elevated"}` },
		Now: func() time.Time {
			now = now.Add(time.Millisecond)
			return now
		},
	})
	if result.OK || len(called) != 1 || result.DurationMS != 1 || !strings.Contains(result.Error, "race failed") {
		t.Fatalf("risk failure result=%+v commands=%v", result, called)
	}
}
