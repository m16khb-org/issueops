package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	"time"

	"issueops/cmd/issueops/commandstep"
	"issueops/internal/adapter/verification"
)

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) selfverify.StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) selfverify.StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, outputBudget, name, args...)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return commandstep.TailWithBudget(s, max)
}

func indentLines(s string) string {
	return commandstep.IndentLines(s)
}
