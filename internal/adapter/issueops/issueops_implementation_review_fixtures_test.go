package issueops

import (
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
)

// RecordIssueOpsImplementationReview는 verdict와 실질 내용(findings/evidence
// 각 1개 이상)을 요구한다. reviewer_* 필드는 감사 기록으로만 저장한다.
func RecordIssueOpsImplementationReview(stateRoot, id string, req issueops.IssueOpsImplementationReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsImplementationReview(stateRoot, id, req, nil)
}

// RecordIssueOpsImplementationReviewWithActor는 활성 lease가 있으면 그 holder만
// 기록하게 한다. 리뷰를 수행한 모델은 reviewer_* 필드에 남고, 기록 권한은
// 사이클 owner에게 있다.
func RecordIssueOpsImplementationReviewWithActor(stateRoot, id string, req issueops.IssueOpsImplementationReviewRequest, actor issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsImplementationReview(stateRoot, id, req, &actor)
}

func recordIssueOpsImplementationReview(stateRoot, id string, req issueops.IssueOpsImplementationReviewRequest, actor *issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.RecordImplementationReview(NewEvidenceReviewStore(actor, testChangeReader().ChangeFingerprint), stateRoot, id, req)
}
