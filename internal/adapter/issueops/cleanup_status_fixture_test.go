package issueops

import (
	cleanupapp "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
)

func IssueOpsCleanupStatusForRecord(record model.IssueOpsRecord, req model.IssueOpsCleanupStatusRequest) model.IssueOpsCleanupStatus {
	return (cleanupapp.StructuralStatus{Environment: CleanupStatusEnvironment{RunGit: GitCmd, ReadGit: GitOut}}).ForRecord(record, req)
}
