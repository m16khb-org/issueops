package commandstep

import (
	selfverify "issueops/internal/contract/selfverify"

	"fmt"
	"strings"
	"time"

	selfverifydomain "issueops/internal/domain/selfverify"
)

func BudgetCommandOutput(s string, budget int) (string, bool, int) {
	return selfverifydomain.BudgetCommandOutput(s, budget)
}

func CombineFailedStep(label string, started time.Time, child selfverify.StepResult, stdoutParts []string, commands []string, outputBudget int) selfverify.StepResult {
	return selfverifydomain.CombineFailedStep(label, time.Since(started).Milliseconds(), child, stdoutParts, commands, outputBudget)
}

func AssertionStep(label string, started time.Time, errs []string) selfverify.StepResult {
	return selfverifydomain.AssertionStep(label, time.Since(started).Milliseconds(), errs)
}

func AssertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string, outputBudget int) selfverify.StepResult {
	return selfverifydomain.AssertionStepWithOutput(label, time.Since(started).Milliseconds(), errs, stdoutParts, commands, outputBudget)
}

func FailedStep(label string, err error) selfverify.StepResult {
	return selfverifydomain.FailedStep(label, err)
}

func PrintStep(step selfverify.StepResult) {
	if step.OK {
		fmt.Printf("→ %s ok (%dms)\n", step.Label, step.DurationMS)
		return
	}
	fmt.Printf("→ %s failed (%dms): %s\n", step.Label, step.DurationMS, step.Error)
	if step.Stdout != "" {
		fmt.Printf("  stdout:\n%s\n", IndentLines(step.Stdout))
	}
	if step.Stderr != "" {
		fmt.Printf("  stderr:\n%s\n", IndentLines(step.Stderr))
	}
}

func Tail(s string, max int) string {
	out, _, _ := TailWithBudget(s, max)
	return out
}

func TailWithBudget(s string, max int) (string, bool, int) {
	return selfverifydomain.TailWithBudget(s, max)
}

func IndentLines(s string) string {
	lines := splitLines(s)
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
