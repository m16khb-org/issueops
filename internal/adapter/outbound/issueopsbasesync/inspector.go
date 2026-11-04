// Package issueopsbasesync is the git-backed outbound adapter that observes whether
// a cycle worktree still contains its base branch for the base-sync check.
package issueopsbasesync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"issueops/internal/adapter/outbound/gitbin"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

var errGitRunnerRequired = errors.New("base sync git runner is required")

type gitRunner func(context.Context, string, ...string) (int, string, string, error)

// Inspector implements the base-sync port by running git in the cycle worktree
// through an injected runner.
type Inspector struct {
	run gitRunner
}

// NewInspector returns an Inspector that runs git through run, such as RunGit.
// Observe fails if run is nil.
func NewInspector(run gitRunner) *Inspector {
	return &Inspector{run: run}
}

// Observe fetches request.BaseBranch from origin in request.Worktree and returns
// the base and HEAD object IDs, with SyncRequired set when the base tip is not an
// ancestor of HEAD. It errors on missing worktree or base branch, git failures,
// or an unexpected merge-base exit code.
func (i *Inspector) Observe(ctx context.Context, request basesyncport.Request) (basesyncport.Receipt, error) {
	if i == nil || i.run == nil {
		return basesyncport.Receipt{}, errGitRunnerRequired
	}
	worktree := strings.TrimSpace(request.Worktree)
	if worktree == "" {
		return basesyncport.Receipt{}, fmt.Errorf("worktree is required")
	}
	baseBranch := strings.TrimSpace(request.BaseBranch)
	if baseBranch == "" {
		return basesyncport.Receipt{}, fmt.Errorf("base branch is required")
	}
	if err := i.requireSuccess(ctx, worktree, "fetch base branch", "fetch", "--quiet", "origin", baseBranch); err != nil {
		return basesyncport.Receipt{}, err
	}
	baseOID, err := i.requireOutput(ctx, worktree, "read FETCH_HEAD", "rev-parse", "FETCH_HEAD")
	if err != nil {
		return basesyncport.Receipt{}, err
	}
	workOID, err := i.requireOutput(ctx, worktree, "read HEAD", "rev-parse", "HEAD")
	if err != nil {
		return basesyncport.Receipt{}, err
	}
	code, _, stderr, err := i.run(ctx, worktree, "merge-base", "--is-ancestor", baseOID, workOID)
	if err != nil {
		return basesyncport.Receipt{}, fmt.Errorf("observe base ancestry: %w", err)
	}
	switch code {
	case 0:
		return basesyncport.Receipt{BaseOID: baseOID, WorkOID: workOID}, nil
	case 1:
		return basesyncport.Receipt{BaseOID: baseOID, WorkOID: workOID, SyncRequired: true}, nil
	default:
		return basesyncport.Receipt{}, fmt.Errorf("observe base ancestry: git exited %d: %s", code, strings.TrimSpace(stderr))
	}
}

func (i *Inspector) requireSuccess(ctx context.Context, worktree, step string, args ...string) error {
	code, _, stderr, err := i.run(ctx, worktree, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	if code != 0 {
		return fmt.Errorf("%s: git exited %d: %s", step, code, strings.TrimSpace(stderr))
	}
	return nil
}

func (i *Inspector) requireOutput(ctx context.Context, worktree, step string, args ...string) (string, error) {
	code, stdout, stderr, err := i.run(ctx, worktree, args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", step, err)
	}
	value := strings.TrimSpace(stdout)
	if code != 0 || value == "" {
		return "", fmt.Errorf("%s: git exited %d: %s", step, code, strings.TrimSpace(stderr))
	}
	return value, nil
}

// RunGit runs git with args in dir with terminal prompts disabled. It returns the
// exit code, stdout, and stderr; a non-zero exit is reported through the code, and
// the error is non-nil only when git could not be run.
func RunGit(ctx context.Context, dir string, args ...string) (int, string, string, error) {
	command := exec.CommandContext(ctx, gitbin.Path(), args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String(), nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String(), nil
	}
	return 0, stdout.String(), stderr.String(), err
}

var _ basesyncport.Inspector = (*Inspector)(nil)
