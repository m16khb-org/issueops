package issueops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"issueops/internal/adapter/outbound/processlease"
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
)

type CleanupLifetimeLock struct {
	StateRoot string
	// Wait bounds how long a busy lock is retried; zero means
	// cleanupLifetimeWait and a negative value refuses at once.
	Wait time.Duration
}

func (s CleanupLifetimeLock) Acquire(ctx context.Context, id string) (cleanupapp.CleanupLifetime, error) {
	normalized, err := normalizeIssueOpsID(id)
	if err != nil {
		return nil, err
	}
	// Keep the physical namespace stable across binary updates; inherited
	// children may still hold descriptors opened by the earlier finish executor.
	directory := filepath.Join(s.StateRoot, "cleanup-finish-locks")
	// QA (Codex runs tool calls in parallel): cleanup status --merged and
	// finish --preview started together, and every one but the first failed
	// at once. A short wait lets concurrent previews take turns; a long-lived
	// holder (an apply, or a child it left behind) still refuses.
	wait := s.Wait
	if wait == 0 {
		wait = cleanupLifetimeWait
	}
	deadline := time.Now().Add(wait)
	for {
		lease, err := processlease.Acquire(ctx, directory, normalized)
		if err == nil {
			return cleanupLifetime{lease}, nil
		}
		if !errors.Is(err, processlease.ErrBusy) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%w: another cleanup command for %s (finish, remote-branch, abandon, or cleanup status --merged), or a process it started, still holds the cleanup lock; let it finish, then rerun", err, normalized)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

const cleanupLifetimeWait = 2 * time.Second

type cleanupLifetime struct{ lease *processlease.Lease }

func (l cleanupLifetime) Context(ctx context.Context) context.Context { return l.lease.Context(ctx) }
func (l cleanupLifetime) Close() error                                { return l.lease.Close() }
func (l cleanupLifetime) Drain(ctx context.Context) (cleanupapp.CleanupLifetime, error) {
	next, err := l.lease.Drain(ctx)
	if err != nil {
		_ = l.lease.Close()
		return nil, err
	}
	return cleanupLifetime{next}, nil
}

func NewCleanupAttempt(operation model.CleanupOperation) (model.IssueOpsCleanupAttempt, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return model.IssueOpsCleanupAttempt{}, err
	}
	return model.IssueOpsCleanupAttempt{Operation: operation, Token: hex.EncodeToString(token), StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}
