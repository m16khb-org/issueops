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

func TestExecutePreservesScopeWithOversizedSuccessAndFailureLogs(t *testing.T) {
	for _, ok := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "failure"}[ok], func(t *testing.T) {
			scope := &contract.Scope{BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40)}
			step := Execute("/repo", ExecuteDeps{
				Plan: func(string) contract.RiskQATierPlan {
					return contract.RiskQATierPlan{Scope: scope, Commands: []string{"go vet ./..."}}
				},
				RenderPlan: func(contract.RiskQATierPlan) string { return "plan" }, Now: time.Now,
				Run: func(string, string) selfverify.StepResult {
					return selfverify.StepResult{OK: ok, Command: "go vet ./...", Label: "vet", Error: "diagnostic", Stdout: strings.Repeat("x", 20000) + "output-end", Stderr: "stderr evidence"}
				},
			})
			if step.OK != ok || !step.StdoutTruncated || len(step.Stdout) > AggregateOutputBudgetBytes || step.StdoutBytes <= AggregateOutputBudgetBytes {
				t.Fatalf("budget metadata: %+v", step)
			}
			for _, want := range []string{scope.BaseSHA, scope.HeadSHA, "output-end"} {
				if !strings.Contains(step.Stdout, want) {
					t.Errorf("lost %q", want)
				}
			}
			if !ok && (step.Stderr != "stderr evidence" || !strings.Contains(step.Error, "diagnostic")) {
				t.Fatalf("lost failure diagnostics: %+v", step)
			}
		})
	}
}

func TestExecuteScopeFailureSurvivesOversizedPlanAndCannotClaimSuccess(t *testing.T) {
	step := Execute("/repo", ExecuteDeps{
		Plan: func(string) contract.RiskQATierPlan {
			return contract.RiskQATierPlan{Scope: &contract.Scope{Error: "range unavailable"}, Commands: []string{"go vet ./..."}}
		},
		RenderPlan: func(contract.RiskQATierPlan) string { return strings.Repeat("p", 20000) }, Now: time.Now,
		Run: func(string, string) selfverify.StepResult {
			t.Fatal("scope failure ran a command")
			return selfverify.StepResult{}
		},
	})
	if step.OK || step.Error != "range unavailable" || !strings.Contains(step.Stdout, `"error":"range unavailable"`) || !step.StdoutTruncated || len(step.Stdout) > AggregateOutputBudgetBytes {
		t.Fatalf("scope failure lost: %+v", step)
	}
}
