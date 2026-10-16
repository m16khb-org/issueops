package issueops

import (
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
)

func stopCleanupWorkspaceProcesses(root string, preview []model.CleanupWorkspaceProcess, deps CleanupProcessDeps) ([]model.CleanupWorkspaceProcess, error) {
	return (cleanupapp.WorkspaceProcessStopper{Processes: NewCleanupProcessController(deps)}).Stop(root, preview)
}

func cleanupWorkspaceGatesForRecord(record model.IssueOpsRecord, root string, processes CleanupProcessDeps) (cleanupapp.FinishWorkspaceObservation, []string) {
	return NewCleanupWorkspaceCleaner(processes).Observe(root, record.Repo)
}
