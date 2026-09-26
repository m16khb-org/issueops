package riskqa

import (
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
	stdoutParts := []string{planJSON}
	commands := []string{}
	if len(plan.Commands) == 0 {
		return selfverifycontract.StepResult{
			Label:      "risk QA tier",
			OK:         true,
			DurationMS: deps.Now().Sub(started).Milliseconds(),
			Stdout:     planJSON,
		}
	}
	for _, command := range plan.Commands {
		step := deps.Run(root, command)
		commands = append(commands, step.Command)
		stdoutParts = append(stdoutParts, step.Stdout)
		if !step.OK {
			return selfverifydomain.CombineFailedStep("risk QA tier", deps.Now().Sub(started).Milliseconds(), step, stdoutParts, commands, AggregateOutputBudgetBytes)
		}
	}
	stdoutText, stdoutTruncated, stdoutBytes := selfverifydomain.TailWithBudget(strings.Join(stdoutParts, "\n"), AggregateOutputBudgetBytes)
	return selfverifycontract.StepResult{
		Label:           "risk QA tier",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      deps.Now().Sub(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
