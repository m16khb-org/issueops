package riskqa

import (
	"encoding/json"
	"strings"
	"time"

	riskqacontract "issueops/internal/contract/riskqa"
	selfverifycontract "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"
)

const AggregateOutputBudgetBytes = 8 * 1024

type ExecuteDeps struct {
	Plan       func(string) riskqacontract.RiskQATierPlan
	Run        func(string, string) selfverifycontract.StepResult
	RenderPlan func(riskqacontract.RiskQATierPlan) string
	Now        func() time.Time
}

func Execute(root string, deps ExecuteDeps) selfverifycontract.StepResult {
	started := deps.Now()
	plan := deps.Plan(root)
	planJSON := deps.RenderPlan(plan)
	// Reserve scope evidence independently of arbitrarily large command logs.
	suffix := ""
	if plan.Scope != nil {
		encoded, _ := json.Marshal(struct {
			Scope *riskqacontract.Scope `json:"scope"`
		}{plan.Scope})
		suffix = "\n" + string(encoded)
	}
	budget := AggregateOutputBudgetBytes - len(suffix)
	stdoutParts := []string{planJSON}
	commands := []string{}
	if (plan.Scope != nil && plan.Scope.Error != "") || len(plan.Commands) == 0 {
		stdout, truncated, size := selfverifydomain.TailWithBudget(planJSON, budget)
		errorText := ""
		if plan.Scope != nil {
			errorText = plan.Scope.Error
		}
		return selfverifycontract.StepResult{
			Label:           "risk QA tier",
			OK:              errorText == "",
			Error:           errorText,
			DurationMS:      deps.Now().Sub(started).Milliseconds(),
			Stdout:          stdout + suffix,
			StdoutBytes:     size + len(suffix),
			StdoutTruncated: truncated,
		}
	}
	for _, command := range plan.Commands {
		step := deps.Run(root, command)
		commands = append(commands, step.Command)
		stdoutParts = append(stdoutParts, step.Stdout)
		if !step.OK {
			result := selfverifydomain.CombineFailedStep("risk QA tier", deps.Now().Sub(started).Milliseconds(), step, stdoutParts, commands, budget)
			result.Stdout += suffix
			result.StdoutBytes += len(suffix)
			return result
		}
	}
	stdoutText, stdoutTruncated, stdoutBytes := selfverifydomain.TailWithBudget(strings.Join(stdoutParts, "\n"), budget)
	return selfverifycontract.StepResult{
		Label:           "risk QA tier",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      deps.Now().Sub(started).Milliseconds(),
		Stdout:          stdoutText + suffix,
		StdoutBytes:     stdoutBytes + len(suffix),
		StdoutTruncated: stdoutTruncated,
	}
}
