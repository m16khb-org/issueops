package issueops

import (
	"strings"

	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
)

func IssueOpsPRReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := cycleapp.BaseImplementationMissing(record)
	if strings.TrimSpace(record.WorktreePath) == "" {
		missing = append(missing, "worktree_path")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueopsdomain.PlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean")
	}
	// non-strict readiness는 git 실행 컨텍스트가 없어 현재 fingerprint를 알 수
	// 없다 — staleness 판정은 strict(IssueOpsStrictPRReadinessWithState)와
	// create-pr 경로가 소유한다.
	if reviewMissing := implementationReviewMissing(record, ""); reviewMissing != "" {
		missing = append(missing, reviewMissing)
	}
	if docsMissing := projectDocsReviewMissing(record, ""); docsMissing != "" {
		missing = append(missing, docsMissing)
	}
	// schema_evidence는 변경 집합을 읽어야 활성 여부를 알 수 있다. 이 표면은
	// record만으로 판정하는 경량 경로이므로 그 게이트는 strict가 소유한다.
	if cycleapp.HasUnresolvedContractFeedback(record) {
		missing = append(missing, "contract_feedback_issue_update")
	}
	missing = stringlist.UniqueSorted(missing)
	cleanup := IssueOpsCleanupStatusForRecord(record, issueops.IssueOpsCleanupStatusRequest{Merged: false})
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
