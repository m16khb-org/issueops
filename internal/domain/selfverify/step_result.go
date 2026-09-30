package selfverify

import (
	"strings"

	contract "issueops/internal/contract/selfverify"
)

func CombineFailedStep(label string, durationMS int64, child contract.StepResult, stdoutParts, commands []string, outputBudget int) contract.StepResult {
	stdoutText, stdoutTruncated, stdoutBytes := TailWithBudget(strings.Join(stdoutParts, "\n"), outputBudget)
	step := contract.StepResult{
		Label:           label,
		Command:         strings.Join(commands, " && "),
		OK:              false,
		DurationMS:      durationMS,
		Stdout:          stdoutText,
		Stderr:          child.Stderr,
		StdoutBytes:     stdoutBytes,
		StderrBytes:     child.StderrBytes,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: child.StderrTruncated,
		Error:           child.Label + ": " + child.Error,
	}
	if step.Error == child.Label+": " {
		step.Error = child.Label + " failed"
	}
	return step
}
