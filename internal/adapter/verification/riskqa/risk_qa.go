// Package riskqa is the adapter that plans and executes the risk-based QA tiers
// (race tests and vet) for self-verify, shelling out to go and git.
package riskqa

import (
	"fmt"
	"time"

	verification "issueops/internal/adapter/verification"
	riskqaapp "issueops/internal/application/riskqa"
	riskqacontract "issueops/internal/contract/riskqa"
	verifycontract "issueops/internal/contract/selfverify"
	riskqadomain "issueops/internal/domain/riskqa"
	verifydomain "issueops/internal/domain/selfverify"
)

const (
	selfVerifyCommandOutputBudgetBytes = 32 * 1024

	riskQARaceTimeout = 10 * time.Minute
	riskQAVetTimeout  = 120 * time.Second
)

// ValidateForSelfVerifyWithScope plans the QA tier from the paths changed since
// baseRef, executes it, and returns the step result along with whether the plan
// covers the full go test run.
func ValidateForSelfVerifyWithScope(root, baseRef string) (verifycontract.StepResult, bool) {
	deps := defaultDeps()
	plan := planWithScope(root, baseRef)
	deps.Plan = func(string) riskqacontract.TierPlan { return plan }
	return ValidateWithDeps(root, deps), riskqadomain.CoversFullGoTest(plan)
}

func defaultDeps() riskqaapp.ExecuteDeps {
	return riskqaapp.ExecuteDeps{
		Plan: Plan,
		Run: func(root string, command string) verifycontract.StepResult {
			switch command {
			case riskqadomain.FullRaceCommand:
				return verification.Run(root, "risk QA race test", riskQARaceTimeout, "", selfVerifyCommandOutputBudgetBytes, "go", "test", "-race", "./...", "-count=1")
			case "go vet ./...":
				return verification.Run(root, "risk QA static vet", riskQAVetTimeout, "", selfVerifyCommandOutputBudgetBytes, "go", "vet", "./...")
			default:
				return verifydomain.FailedStep("risk QA tier", fmt.Errorf("unknown risk QA command %q", command))
			}
		},
	}
}

// ValidateWithDeps executes the risk QA plan with deps, defaulting RenderPlan to
// PlanJSON and Now to time.Now.
func ValidateWithDeps(root string, deps riskqaapp.ExecuteDeps) verifycontract.StepResult {
	if deps.RenderPlan == nil {
		deps.RenderPlan = PlanJSON
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return riskqaapp.Execute(root, deps)
}
