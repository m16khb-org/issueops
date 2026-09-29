package issueopsapp

import (
	"time"

	"issueops/cmd/issueops/commandstep"
	"issueops/internal/adapter/verification"
)

type StepResult = commandstep.StepResult

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return verification.Run(dir, label, timeout, stdin, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func failedStep(label string, err error) StepResult {
	return commandstep.FailedStep(label, err)
}

func printStep(step StepResult) {
	commandstep.PrintStep(step)
}
