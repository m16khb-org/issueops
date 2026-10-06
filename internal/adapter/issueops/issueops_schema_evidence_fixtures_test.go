package issueops

import (
	cycleapp "issueops/internal/application/issueopscycle"
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
)

// RecordIssueOpsSchemaEvidence는 스키마·마이그레이션·엔티티 변경 사이클의
// 실측 근거를 기록한다. 인덱스 현황이나 row count처럼 실제 데이터베이스에서
// 관찰한 값과 그 출처를 함께 요구한다 — 출처 없는 수치는 추정과 구분되지
// 않기 때문이다. 관찰이 불가능하면 근거를 적어 waive한다.
func RecordIssueOpsSchemaEvidence(stateRoot, id string, req issueops.IssueOpsSchemaEvidenceRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsSchemaEvidence(stateRoot, id, req, nil)
}

// RecordIssueOpsSchemaEvidenceWithActor는 활성 lease가 있으면 그 holder만
// 기록하게 한다.
func RecordIssueOpsSchemaEvidenceWithActor(stateRoot, id string, req issueops.IssueOpsSchemaEvidenceRequest, actor issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsSchemaEvidence(stateRoot, id, req, &actor)
}

func recordIssueOpsSchemaEvidence(stateRoot, id string, req issueops.IssueOpsSchemaEvidenceRequest, actor *issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.RecordSchemaEvidence(NewEvidenceReviewStore(actor, testChangeReader().ChangeFingerprint), stateRoot, id, req)
}

// schemaEvidenceMissing은 변경 집합에 스키마 파일이 있을 때만 활성화되는
// 조건부 게이트다. DB를 쓰지 않는 사이클에서는 아무것도 요구하지 않는다.
func schemaEvidenceMissing(record issueops.IssueOpsRecord, currentFingerprint string) string {
	return cycleapp.ObservedSchemaEvidenceMissing(record, true, nil, currentFingerprint, testChangeReader().ChangedPaths)
}
