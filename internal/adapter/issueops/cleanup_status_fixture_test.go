package issueops

import (
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
)

func IssueOpsCleanupStatusForRecord(record model.Record, req model.CleanupStatusRequest) model.CleanupStatus {
	return (cleanupapp.StructuralStatus{Environment: CleanupStatusEnvironment{RunGit: GitCmd, ReadGit: GitOut}}).ForRecord(record, req)
}
