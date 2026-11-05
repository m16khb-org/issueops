package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
)

// CleanupFinishFingerprint returns the hex SHA-256 of the JSON-encoded inventory, used to
// bind a finish apply to the preview it observed.
func CleanupFinishFingerprint(inventory model.CleanupFinishInventory) (string, error) {
	raw, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// CleanupFinishRuntime runs the git, workspace-observation and process-stop steps of
// cleanup finish. A nil RunGit falls back to the default sync-base git runner, and
// Processes overrides process observation and termination.
type CleanupFinishRuntime struct {
	RunGit    func(string, ...string) (int, string)
	Processes CleanupProcessDeps
}

// Git runs git in dir through RunGit, or the default runner when RunGit is nil.
func (r CleanupFinishRuntime) Git(ctx context.Context, dir string, args ...string) (int, string) {
	if r.RunGit != nil {
		return r.RunGit(dir, args...)
	}
	return defaultExecutionSyncBaseGit(ctx, dir, args...)
}

// Workspace observes the worktree at root for the record and returns the observation plus
// its residual notes.
func (r CleanupFinishRuntime) Workspace(ctx context.Context, record model.Record, root string) (cleanupapp.FinishWorkspaceObservation, []string) {
	return newCleanupWorkspaceCleaner(r.Processes).Observe(root, record.Repo)
}

// Stop terminates the workspace processes listed for the inventory worktree and returns
// those that remain.
func (r CleanupFinishRuntime) Stop(ctx context.Context, inventory model.CleanupFinishInventory, processes []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, error) {
	return newCleanupWorkspaceCleaner(r.Processes).Stop(inventory.WorktreeRoot, processes)
}
