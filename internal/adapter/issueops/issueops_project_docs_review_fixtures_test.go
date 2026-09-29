package issueops

import (
	cycleapp "issueops/internal/application/issueopscycle"
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordIssueOpsProjectDocsReview는 publication 직전 project-doc 반영 판정을
// 기록한다. verdict가 updated면 적어 낸 문서가 실제 변경 집합 안에 있어야
// 하므로, 문서를 고치지 않고 "갱신했다"고 기록하는 경로가 막힌다.
func RecordIssueOpsProjectDocsReview(stateRoot, id string, req IssueOpsProjectDocsReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsProjectDocsReview(stateRoot, id, req, nil)
}

// RecordIssueOpsProjectDocsReviewWithActor는 활성 lease가 있으면 그 holder만
// 기록하게 한다.
func RecordIssueOpsProjectDocsReviewWithActor(stateRoot, id string, req IssueOpsProjectDocsReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsProjectDocsReview(stateRoot, id, req, &actor)
}

func recordIssueOpsProjectDocsReview(stateRoot, id string, req IssueOpsProjectDocsReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.RecordProjectDocsReview(reviewport.ProjectDocsReviewStore{
		EvidenceReviewStore: NewEvidenceReviewStore(actor, testChangeReader().ChangeFingerprint),
		ChangedPaths:        testChangeReader().ChangedPaths,
		Root:                ReviewDocumentPaths{}.Root,
		RelativePath:        ReviewDocumentPaths{}.RelativePath,
		FileExists:          ReviewDocumentPaths{}.FileExists,
	}, stateRoot, id, req)
}

// projectDocsReviewMissing은 publication 게이트 판정이다. implementation review와
// 달리 execution mode도, execution lease 유무도 가리지 않는다 — 어떤 경로로
// implement 이후 phase에 왔든 운영 문서에 남길 결정을 만들 수 있기 때문이다.
func projectDocsReviewMissing(record issueops.IssueOpsRecord, currentFingerprint string) string {
	return cycleapp.ProjectDocsReviewMissing(record, currentFingerprint)
}
