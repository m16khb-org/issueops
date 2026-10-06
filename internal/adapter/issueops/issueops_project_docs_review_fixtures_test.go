package issueops

import (
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordIssueOpsProjectDocsReview는 publication 직전 project-doc 반영 판정을
// 기록한다. verdict가 updated면 적어 낸 문서가 실제 변경 집합 안에 있어야
// 하므로, 문서를 고치지 않고 "갱신했다"고 기록하는 경로가 막힌다.
func RecordIssueOpsProjectDocsReview(stateRoot, id string, req issueops.IssueOpsProjectDocsReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsProjectDocsReview(stateRoot, id, req, nil)
}

// RecordIssueOpsProjectDocsReviewWithActor는 활성 lease가 있으면 그 holder만
// 기록하게 한다.
func RecordIssueOpsProjectDocsReviewWithActor(stateRoot, id string, req issueops.IssueOpsProjectDocsReviewRequest, actor issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsProjectDocsReview(stateRoot, id, req, &actor)
}

func recordIssueOpsProjectDocsReview(stateRoot, id string, req issueops.IssueOpsProjectDocsReviewRequest, actor *issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.RecordProjectDocsReview(reviewport.ProjectDocsReviewStore{
		EvidenceReviewStore: NewEvidenceReviewStore(actor, testChangeReader().ChangeFingerprint),
		ChangedPaths:        testChangeReader().ChangedPaths,
		Root:                ReviewDocumentPaths{}.Root,
		RelativePath:        ReviewDocumentPaths{}.RelativePath,
		FileExists:          ReviewDocumentPaths{}.FileExists,
	}, stateRoot, id, req)
}
