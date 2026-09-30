package issueops

import (
	"context"
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func stopCleanupWorkspaceProcesses(root string, preview []model.CleanupWorkspaceProcess, excluded map[int]bool, deps CleanupProcessDeps) ([]model.CleanupWorkspaceProcess, error) {
	return (cleanupapp.WorkspaceProcessStopper{Processes: NewCleanupProcessController(deps)}).Stop(root, preview, excluded)
}

func cleanupWorkspaceGatesForRecord(ctx context.Context, record model.IssueOpsRecord, root string, processes CleanupProcessDeps, orca port.CleanupOrcaTerminals) (cleanupapp.FinishWorkspaceObservation, []string) {
	return NewCleanupWorkspaceCleaner(processes, orca).Observe(ctx, record, root)
}
