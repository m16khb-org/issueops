package verification

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	contract "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

func Run(dir, label string, timeout time.Duration, stdin string, outputBudget int, name string, args ...string) contract.StepResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	stdoutText, stdoutTruncated, stdoutBytes := domain.BudgetCommandOutput(stdout.String(), outputBudget)
	stderrText, stderrTruncated, stderrBytes := domain.BudgetCommandOutput(stderr.String(), outputBudget)
	step := contract.StepResult{
		Label:           label,
		Command:         strings.Join(append([]string{name}, args...), " "),
		OK:              err == nil,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		Stderr:          stderrText,
		StdoutBytes:     stdoutBytes,
		StderrBytes:     stderrBytes,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
	}
	if ctx.Err() == context.DeadlineExceeded {
		step.OK = false
		step.Error = "timeout after " + timeout.String()
	} else if err != nil {
		step.Error = err.Error()
	}
	return step
}
