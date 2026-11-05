// Package riskqa plans the risk-based QA tier for a change and executes the
// planned commands. It is an application-layer service; path scoping and
// command running are injected.
package riskqa

import (
	"encoding/json"
	"strings"
	"time"

	riskqacontract "issueops/internal/contract/riskqa"
	selfverifycontract "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024

// ExecuteDeps supplies the effects Execute needs: plan the tier for a root,
// run one planned command, render the plan, and read the clock.
type ExecuteDeps struct {
	Plan       func(string) riskqacontract.TierPlan
	Run        func(string, string) selfverifycontract.StepResult
	RenderPlan func(riskqacontract.TierPlan) string
	Now        func() time.Time
}

// Execute plans the risk QA tier for root and runs each planned command in
// order, stopping at the first failure. It returns one step result whose
// stdout is the plan plus command output, trimmed to the aggregate output
// budget with the scope JSON always kept. A scope error or an empty command
// list returns without running anything.
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
	budget := aggregateOutputBudgetBytes - len(suffix)
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
