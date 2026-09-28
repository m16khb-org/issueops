package issueops

import (
	cleanupapp "issueops/internal/application/issueopscleanup"
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
)

func IssueOpsPRReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := stringlist.UniqueSorted(cycleapp.LocalPRReadinessMissing(record, issueOpsReadinessObservations()))
	cleanup := (cleanupapp.StructuralStatus{Environment: CleanupStatusEnvironment{RunGit: GitCmd, ReadGit: GitOut}}).ForRecord(record, issueops.IssueOpsCleanupStatusRequest{Merged: false})
	return issueops.IssueOpsReadiness{
		OK:             true,
		Ready:          len(missing) == 0,
		Missing:        missing,
		Warnings:       issueopsdomain.PRReadinessWarnings(record),
		CleanupReady:   cleanup.Ready,
		CleanupMissing: cleanup.Missing,
		IssueURL:       record.IssueURL,
		PlanPath:       record.PlanPath,
		WorktreePath:   record.WorktreePath,
		Branch:         record.Branch,
	}
}
