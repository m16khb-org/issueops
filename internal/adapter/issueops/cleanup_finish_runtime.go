package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

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
