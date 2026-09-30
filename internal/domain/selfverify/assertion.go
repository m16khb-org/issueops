package selfverify

import (
	"strings"

	contract "issueops/internal/contract/selfverify"
)

func AssertionStep(label string, durationMS int64, errs []string) contract.StepResult {
	step := contract.StepResult{Label: label, OK: len(errs) == 0, DurationMS: durationMS}
	if len(errs) > 0 {
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

func AssertionStepWithOutput(label string, durationMS int64, errs []string, stdoutParts, commands []string, outputBudget int) contract.StepResult {
	step := AssertionStep(label, durationMS, errs)
	step.Command = strings.Join(commands, " && ")
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = TailWithBudget(strings.Join(stdoutParts, "\n"), outputBudget)
	return step
}

func FailedStep(label string, err error) contract.StepResult {
	return contract.StepResult{Label: label, OK: false, Error: err.Error()}
}
