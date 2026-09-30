package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	"time"

	"issueops/cmd/issueops/commandstep"
	"issueops/internal/adapter/verification"
)

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) selfverify.StepResult {
	return verification.Run(dir, label, timeout, stdin, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func failedStep(label string, err error) selfverify.StepResult {
	return commandstep.FailedStep(label, err)
}

func printStep(step selfverify.StepResult) {
	commandstep.PrintStep(step)
}
