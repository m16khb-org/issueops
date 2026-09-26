package riskqa

import (
	"fmt"
	"time"

	"issueops/cmd/issueops/commandstep"
	riskqaapp "issueops/internal/application/riskqa"
	riskqacontract "issueops/internal/contract/riskqa"
	riskqadomain "issueops/internal/domain/riskqa"
)

const (
	selfVerifyCommandOutputBudgetBytes   = 32 * 1024
	selfVerifyAggregateOutputBudgetBytes = riskqaapp.AggregateOutputBudgetBytes
	riskQARaceTimeout                    = 10 * time.Minute
	riskQAVetTimeout                     = 120 * time.Second
	fullRaceCommand                      = riskqadomain.FullRaceCommand
)

type RiskQATierPlan = riskqacontract.RiskQATierPlan

type StepResult = commandstep.StepResult

func Validate(root string) StepResult {
	return ValidateWithDeps(root, defaultDeps())
}

func ValidateForSelfVerify(root string) (StepResult, bool) {
	deps := defaultDeps()
	plan := deps.Plan(root)
	deps.Plan = func(string) RiskQATierPlan { return plan }
	return ValidateWithDeps(root, deps), riskqadomain.CoversFullGoTest(plan)
}

func defaultDeps() Deps {
	return Deps{
		Plan: Plan,
		Run: func(root string, command string) StepResult {
			switch command {
			case fullRaceCommand:
				return commandstep.Run(root, "risk QA race test", riskQARaceTimeout, "", selfVerifyCommandOutputBudgetBytes, "go", "test", "-race", "./...", "-count=1")
			case "go vet ./...":
				return commandstep.Run(root, "risk QA static vet", riskQAVetTimeout, "", selfVerifyCommandOutputBudgetBytes, "go", "vet", "./...")
			default:
				return commandstep.FailedStep("risk QA tier", fmt.Errorf("unknown risk QA command %q", command))
			}
		},
	}
}

type Deps = riskqaapp.ExecuteDeps

func ValidateWithDeps(root string, deps Deps) StepResult {
	if deps.RenderPlan == nil {
		deps.RenderPlan = PlanJSON
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return riskqaapp.Execute(root, deps)
}
