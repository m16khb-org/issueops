package issueops

import (
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
)

func stopCleanupWorkspaceProcesses(root string, preview []model.CleanupWorkspaceProcess, deps CleanupProcessDeps) ([]model.CleanupWorkspaceProcess, error) {
	return (cleanupapp.WorkspaceProcessStopper{Processes: newCleanupProcessController(deps)}).Stop(root, preview)
}

func cleanupWorkspaceGatesForRecord(record model.Record, root string, processes CleanupProcessDeps) (cleanupapp.FinishWorkspaceObservation, []string) {
	return newCleanupWorkspaceCleaner(processes).Observe(root, record.Repo)
}
