package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	"time"

	"issueops/cmd/issueops/commandstep"
	"issueops/internal/adapter/verification"
	selfverifydomain "issueops/internal/domain/selfverify"
)

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) selfverify.StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) selfverify.StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, outputBudget, name, args...)
}

func mergeEnvOverrides(base []string, overrides []string) []string {
	return selfverifydomain.MergeEnvOverrides(base, overrides)
}

func envEntryKey(entry string) (string, bool) {
	return selfverifydomain.EnvEntryKey(entry)
}

func budgetCommandOutput(s string, budget int) (string, bool, int) {
	return commandstep.BudgetCommandOutput(s, budget)
}

func combineFailedStep(label string, started time.Time, child selfverify.StepResult, stdoutParts []string, commands []string) selfverify.StepResult {
	return commandstep.CombineFailedStep(label, started, child, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStep(label string, started time.Time, errs []string) selfverify.StepResult {
	return commandstep.AssertionStep(label, started, errs)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) selfverify.StepResult {
	return commandstep.AssertionStepWithOutput(label, started, errs, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func tail(s string, max int) string {
	return commandstep.Tail(s, max)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return commandstep.TailWithBudget(s, max)
}

func indentLines(s string) string {
	return commandstep.IndentLines(s)
}
