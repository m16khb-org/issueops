package issueops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"time"

	"issueops/internal/adapter/outbound/processlease"
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
)

type CleanupLifetimeLock struct{ StateRoot string }

func (s CleanupLifetimeLock) Acquire(ctx context.Context, id string) (cleanupapp.CleanupLifetime, error) {
	normalized, err := normalizeIssueOpsID(id)
	if err != nil {
		return nil, err
	}
	// Keep the physical namespace stable across binary updates; inherited
	// children may still hold descriptors opened by the earlier finish executor.
	lease, err := processlease.Acquire(ctx, filepath.Join(s.StateRoot, "cleanup-finish-locks"), normalized)
	if err != nil {
		return nil, err
	}
	return cleanupLifetime{lease}, nil
}

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
