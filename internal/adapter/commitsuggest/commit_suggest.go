// Package commitsuggest provides the git-backed effects for commit message
// suggestions: root normalization and diff reading. It is an adapter-layer
// package.
package commitsuggest

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"issueops/internal/adapter/outbound/gitbin"
)

// diffWaitDelay bounds how long Diff keeps waiting for the output pipe once ctx
// ends (and git is killed) or once git exits, whichever comes first. A process
// git started can still hold the pipe: on Windows the killed process can be a
// launcher whose child keeps running, and a pager or hook can outlive git.
// Without the bound Output would wait for that process.
const diffWaitDelay = 2 * time.Second

// Effects runs the commit suggestion side effects. Normalize resolves the
// repository root path.
type Effects struct{ Normalize func(string) (string, error) }

// NormalizeRoot resolves root through the injected Normalize function.
func (effects Effects) NormalizeRoot(root string) (string, error) { return effects.Normalize(root) }

// Diff returns the output of `git diff` in root, restricted to staged changes
// when staged is true. git runs with --no-optional-locks, so killing it does
// not leave .git/index.lock behind. When ctx ends before git finishes, Diff
// kills git and returns ctx.Err() instead of the partial output. When git exits
// 0 while a process it started still holds the pipe, Diff returns the output
// read until diffWaitDelay closed the pipe and no error.
func (Effects) Diff(ctx context.Context, root string, staged bool) (string, error) {
	args := []string{"--no-optional-locks", "-C", root, "diff"}
	if staged {
		args = append(args, "--cached")
	}
	command := exec.CommandContext(ctx, gitbin.Path(), args...)
	command.WaitDelay = diffWaitDelay
	output, err := command.Output()
	if err != nil && ctx.Err() != nil {
		return "", ctx.Err()
	}
	if errors.Is(err, exec.ErrWaitDelay) && command.ProcessState.Success() {
		return string(output), nil
	}
	return string(output), err
}
