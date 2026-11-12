// Package lintdiagnose provides the process effects for lint failure
// diagnosis: root normalization and command execution. It is an adapter-layer
// package.
package lintdiagnose

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

// runWaitDelay bounds how long Run keeps waiting for the output pipes once ctx
// ends (and the command is killed) or once the command exits, whichever comes
// first. A process the command started can still hold the pipes: on Windows
// the killed process can be a launcher whose child keeps running, and any
// command can leave a background child behind. Without the bound
// CombinedOutput would wait for that child.
const runWaitDelay = 2 * time.Second

// Effects runs the lint diagnosis side effects. Normalize resolves the
// repository root path.
type Effects struct{ Normalize func(string) (string, error) }

// NormalizeRoot resolves root through the injected Normalize function.
func (effects Effects) NormalizeRoot(root string) (string, error) { return effects.Normalize(root) }

// Run executes argv in root and returns its combined stdout and stderr, the
// exit code, and whether the command failed. A start failure returns -1. The
// error is non-nil only when ctx ends before the command does: Run then kills
// the command and returns ctx.Err(). Only the started process is killed, not
// the processes it started; runWaitDelay keeps Run from waiting on them. A
// command that exits while a child still holds the pipes keeps its own exit
// status, with the output read until runWaitDelay closed the pipes.
func (Effects) Run(ctx context.Context, root string, argv []string) (string, int, bool, error) {
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = root
	command.WaitDelay = runWaitDelay
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0, false, nil
	}
	if ctx.Err() != nil {
		return string(output), -1, true, ctx.Err()
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		code := command.ProcessState.ExitCode()
		return string(output), code, code != 0, nil
	}
	if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
		return string(output), exitError.ExitCode(), true, nil
	}
	return string(output), -1, true, nil
}
