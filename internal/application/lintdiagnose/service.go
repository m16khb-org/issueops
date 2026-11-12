package lintdiagnose

import (
	"context"
	"errors"
	"fmt"
	"time"

	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	domain "issueops/internal/domain/lintdiagnose"
)

// defaultCommandTimeout bounds one command run when Service.CommandTimeout is
// zero. It guards against a command that hangs; it is not a lint budget, since
// lint_diagnose runs any non-empty argv (skills send test commands through
// it). It sits above the 10-minute timeout in .golangci.yml, the slowest lint
// this repository configures.
const defaultCommandTimeout = 15 * time.Minute

type effects interface {
	NormalizeRoot(string) (string, error)
	// Run returns a non-nil error only when ctx ends before the command does;
	// the error wraps ctx.Err().
	Run(ctx context.Context, root string, argv []string) (output string, exitCode int, failed bool, err error)
}

// Service diagnoses failing commands using its Effects. CommandTimeout bounds
// one command run; zero means 15 minutes.
type Service struct {
	Effects        effects
	CommandTimeout time.Duration
}

// Diagnose validates and runs req.CommandArgv in the normalized repo root. A
// passing command returns without a prompt; a failing one carries the prompt
// built from the output tail, and the host agent produces the actual diagnosis.
// The command runs under ctx and the CommandTimeout bound. When either ends the
// run, Diagnose returns an error that wraps the context error and says whether
// the command timed out at the CommandTimeout bound or was stopped by the
// caller (ctx was cancelled or reached its own deadline first), and builds no
// diagnosis from the partial output. Only the started process is killed, not
// its children.
func (service Service) Diagnose(ctx context.Context, req lintdiagnosecontract.Request) (lintdiagnosecontract.Result, error) {
	root, err := service.Effects.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return lintdiagnosecontract.Result{}, err
	}
	if err := domain.ValidateCommand(req.CommandArgv); err != nil {
		return lintdiagnosecontract.Result{}, err
	}
	timeout := service.CommandTimeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, exitCode, failed, err := service.Effects.Run(runCtx, root, req.CommandArgv)
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			return lintdiagnosecontract.Result{}, fmt.Errorf("lint command timed out after %s: %w", timeout, err)
		}
		return lintdiagnosecontract.Result{}, fmt.Errorf("lint command stopped by the caller: %w", err)
	}
	result := lintdiagnosecontract.Result{
		OK: true, CommandArgv: req.CommandArgv, ExitCode: exitCode, Failed: failed,
	}
	if !failed {
		return result, nil
	}
	result.Prompt = BuildPrompt(exitCode, domain.FailureTail(output))
	result.Diagnosis = "command failed; prompt contains the host-agent judgement request"
	return result, nil
}
