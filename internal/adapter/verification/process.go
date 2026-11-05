// Package verification is the outbound adapter for the self-verification loop:
// it runs commands with a timeout and a bounded output budget, lists tracked Go
// files and gofmt findings, and locates the candidate source file. The
// verification domain decides what the results mean.
package verification

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	contract "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

// Run executes name with args in dir and reports the outcome as a StepResult
// labelled label. It is RunEnv without environment overrides.
func Run(dir, label string, timeout time.Duration, stdin string, outputBudget int, name string, args ...string) contract.StepResult {
	return RunEnv(dir, label, timeout, stdin, nil, outputBudget, name, args...)
}

// BudgetedRunEnv is RunEnv with a fixed output budget, in the shape the
// probes inject as their default command runner.
func BudgetedRunEnv(outputBudget int) func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) contract.StepResult {
	return func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) contract.StepResult {
		return RunEnv(dir, label, timeout, stdin, env, outputBudget, name, args...)
	}
}

// SeededTempState makes a probe's temporary state directory named after the
// probe and the run seed: <prefix>-<seed>-*.
func SeededTempState(prefix string) func(seed int64) (string, error) {
	return func(seed int64) (string, error) {
		return os.MkdirTemp("", fmt.Sprintf("%s-%d-*", prefix, seed))
	}
}

// RunEnv executes name with args in dir under timeout, feeding stdin when it is
// non-empty and merging env over the process environment when it is non-empty.
// Stdout and stderr are each capped at outputBudget. The returned StepResult
// carries the exit status, duration, captured output, and an error message that
// reads "timeout after <timeout>" when the deadline expired.
func RunEnv(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) contract.StepResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = domain.MergeEnvOverrides(os.Environ(), env)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	stdout := domain.NewCommandOutput(outputBudget)
	stderr := domain.NewCommandOutput(outputBudget)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	stdoutText, stdoutTruncated, stdoutBytes := stdout.Result()
	stderrText, stderrTruncated, stderrBytes := stderr.Result()
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
