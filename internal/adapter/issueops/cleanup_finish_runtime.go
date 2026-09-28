package issueops

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"time"

	"issueops/internal/adapter/outbound/processlease"
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type FinishLifetimeLock struct{ StateRoot string }

func (s FinishLifetimeLock) Acquire(ctx context.Context, id string) (cleanupapp.FinishLifetime, error) {
	normalized, err := normalizeIssueOpsID(id)
	if err != nil {
		return nil, err
	}
	lease, err := processlease.Acquire(ctx, filepath.Join(s.StateRoot, "cleanup-finish-locks"), normalized)
	if err != nil {
		return nil, err
	}
	return finishLifetime{lease}, nil
}

type finishLifetime struct{ lease *processlease.Lease }

func (l finishLifetime) Context(ctx context.Context) context.Context { return l.lease.Context(ctx) }
func (l finishLifetime) Close() error                                { return l.lease.Close() }
func (l finishLifetime) Drain(ctx context.Context) (cleanupapp.FinishLifetime, error) {
	next, err := l.lease.Drain(ctx)
	if err != nil {
		_ = l.lease.Close()
		return nil, err
	}
	return finishLifetime{next}, nil
}

func NewCleanupFinishAttempt() (model.IssueOpsCleanupFinishAttempt, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return model.IssueOpsCleanupFinishAttempt{}, err
	}
	return model.IssueOpsCleanupFinishAttempt{Token: hex.EncodeToString(token), StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}
func CleanupFinishFingerprint(inventory model.CleanupFinishInventory) (string, error) {
	raw, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

type CleanupFinishRuntime struct {
	RunGit        func(string, ...string) (int, string)
	Processes     CleanupProcessDeps
	OrcaTerminals port.CleanupOrcaTerminals
}

func (r CleanupFinishRuntime) Git(ctx context.Context, dir string, args ...string) (int, string) {
	if r.RunGit != nil {
		return r.RunGit(dir, args...)
	}
	return defaultExecutionSyncBaseGit(ctx, dir, args...)
}
func (r CleanupFinishRuntime) Workspace(ctx context.Context, record model.IssueOpsRecord, root string) (cleanupapp.FinishWorkspaceObservation, []string) {
	return NewCleanupWorkspaceCleaner(r.Processes, r.OrcaTerminals).Observe(ctx, record, root)
}
func (r CleanupFinishRuntime) Stop(ctx context.Context, inventory model.CleanupFinishInventory, processes []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, int, error) {
	return NewCleanupWorkspaceCleaner(r.Processes, r.OrcaTerminals).Stop(ctx, inventory.WorktreeRoot, processes, inventory.OrcaTerminals, inventory.OrcaRuntimeReady, inventory.OrcaAppPID)
}
