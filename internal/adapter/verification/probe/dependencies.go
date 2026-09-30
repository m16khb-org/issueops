package probe

import (
	"os"
	"strings"
	"time"

	verification "issueops/internal/adapter/verification"

	verifycontract "issueops/internal/contract/selfverify"

	verifydomain "issueops/internal/domain/selfverify"
)

const selfVerifyCommandOutputBudgetBytes = 32 * 1024
const selfVerifyAggregateOutputBudgetBytes = 8 * 1024

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) verifycontract.StepResult {
	return verification.Run(dir, label, timeout, stdin, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) verifycontract.StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}

func runCommandStepEnvWithBudget(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) verifycontract.StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, outputBudget, name, args...)
}

func combineFailedStep(label string, started time.Time, child verifycontract.StepResult, stdoutParts []string, commands []string) verifycontract.StepResult {
	return verifydomain.CombineFailedStep(label, time.Since(started).Milliseconds(), child, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) verifycontract.StepResult {
	return verifydomain.AssertionStepWithOutput(label, time.Since(started).Milliseconds(), errs, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
}

func assertionStep(label string, started time.Time, errs []string) verifycontract.StepResult {
	return verifydomain.AssertionStep(label, time.Since(started).Milliseconds(), errs)
}

func failedStep(label string, err error) verifycontract.StepResult {
	return verifydomain.FailedStep(label, err)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return verifydomain.TailWithBudget(s, max)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
