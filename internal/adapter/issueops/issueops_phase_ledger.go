package issueops

import (
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

// IssueOpsProblemReadiness는 problem phase 완료 여부를 보고한다. problem 완료는
// intent contract만 요구하도록 의도적으로 최소화한다. remote issue나 branch가
// 생기기 전의 자유로운 problem -> grill 전이와 초기 탐색을 보존하기 위해서다.
// issue_url/branch는 grill artifact다.
func IssueOpsProblemReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return cycleapp.ReadinessFromMissing(record, issueopsdomain.IntentMissing(record))
}

// IssueOpsGrillReadiness는 grill phase 완료 여부를 보고한다. 필요한 것은
// issue_url + branch + plan_prep(적용 시) + split_decision + domain_review다.
// 이는 create-issue-after-grill workflow와 현재 plan-entry gate에 맞춰 plan
// 진입을 막는다.
func IssueOpsGrillReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return cycleapp.ReadinessFromMissing(record, issueopsdomain.GrillReadinessMissing(record))
}

// IssueOpsPhaseCompletion은 기존 source-of-truth 필드에서 phase 완료 여부를
// 계산해 ready/artifacts(missing)를 반환한다. 기존 readiness 함수를 색인할 뿐,
// 스스로 source of truth가 되지는 않는다.
func IssueOpsPhaseCompletion(record issueops.IssueOpsRecord, phase issueops.IssueOpsPhase) issueops.IssueOpsReadiness {
	return cycleapp.PhaseCompletion(record, phase, cycleport.PhaseCompletionReadiness{
		Compatibility: IssueOpsCompatibilityReviewReadiness, AISlopClean: IssueOpsAISlopCleanReadiness,
		PR: IssueOpsPRReadiness, RemoteArtifactMissing: issueopsdomain.RemoteArtifactMissing,
	})
}
