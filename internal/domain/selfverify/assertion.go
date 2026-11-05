package selfverify

import (
	"strings"

	contract "issueops/internal/contract/selfverify"
)

// AssertionStep builds a StepResult for an in-process assertion step; it is OK only when errs is empty,
// otherwise the errors are joined with "; " into Error.
func AssertionStep(label string, durationMS int64, errs []string) contract.StepResult {
	step := contract.StepResult{Label: label, OK: len(errs) == 0, DurationMS: durationMS}
	if len(errs) > 0 {
		step.Error = strings.Join(errs, "; ")
	}
	return step
}

// AssertionStepWithOutput extends AssertionStep with the executed commands (joined with " && ") and the
// stdout parts (joined by newlines), keeping only the tail of stdout within outputBudget bytes.
func AssertionStepWithOutput(label string, durationMS int64, errs []string, stdoutParts, commands []string, outputBudget int) contract.StepResult {
	step := AssertionStep(label, durationMS, errs)
	step.Command = strings.Join(commands, " && ")
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = TailWithBudget(strings.Join(stdoutParts, "\n"), outputBudget)
	return step
}

// FailedStep returns a failed StepResult for label carrying err's message. err must be non-nil.
func FailedStep(label string, err error) contract.StepResult {
	return contract.StepResult{Label: label, OK: false, Error: err.Error()}
}
