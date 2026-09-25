package issueops

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops/implementation"
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordIssueOpsImplementationReview는 verdict와 실질 내용(findings/evidence
// 각 1개 이상)을 요구한다. reviewer_* 필드는 감사 기록으로만 저장한다.
func RecordIssueOpsImplementationReview(stateRoot, id string, req IssueOpsImplementationReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsImplementationReview(stateRoot, id, req, nil)
}

// RecordIssueOpsImplementationReviewWithActor는 활성 lease가 있으면 그 holder만
// 기록하게 한다. 리뷰를 수행한 모델은 reviewer_* 필드에 남고, 기록 권한은
// 사이클 owner에게 있다.
func RecordIssueOpsImplementationReviewWithActor(stateRoot, id string, req IssueOpsImplementationReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsImplementationReview(stateRoot, id, req, &actor)
}

func recordIssueOpsImplementationReview(stateRoot, id string, req IssueOpsImplementationReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.RecordImplementationReview(reviewEvidenceStore(actor), stateRoot, id, req)
}

func reviewEvidenceStore(actor *IssueOpsActor) reviewport.EvidenceReviewStore {
	return reviewport.EvidenceReviewStore{
		Read:        ReadIssueOps,
		Fingerprint: implementation.ChangeFingerprint,
		WithLock: func(root, cycleID string, fn func() error) error {
			return withIssueOpsLock(context.Background(), root, cycleID, func(context.Context) error { return fn() })
		},
		ValidateMutation: func(record issueops.IssueOpsRecord) error {
			return validatePostTransferMutation(record, actor)
		},
		Write: writeIssueOps,
		Now:   func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
	}
}

// implementationReviewMissing은 publication 게이트 판정이며 execution이 있는
// 모든 모드에 적용한다. execution이 없는 레코드(준비 전, legacy)만 면제다.
//
// 원래는 orca 모드 한정이었다. 하위 세션을 하네스가 띄우는 그 경로에서만
// 적대 리뷰를 강제할 수 있다고 봤기 때문이다. 9단계 재편(2026-09-04)에서
// direct가 기본 경로가 되고 검증 단계가 이 기록을 만들면서 전제가 바뀌었다.
// orca 한정으로 두면 기본 경로의 리뷰 게이트가 CLI 수준에서 비어 버린다.
//
// currentFingerprint가 비어 있지 않으면 리뷰가 봉인한 fingerprint와 비교해
// stale 리뷰를 거부한다.
func implementationReviewMissing(record issueops.IssueOpsRecord, currentFingerprint string) string {
	evidence := reviewcontract.ReviewGateEvidence{}
	if review := record.ImplementationReview; review != nil {
		evidence = reviewcontract.ReviewGateEvidence{
			Present: true, Verdict: review.Verdict, ReviewedFingerprint: review.ReviewedFingerprint,
		}
	}
	return reviewdomain.ImplementationReviewMissing(record.Execution != nil, evidence, currentFingerprint)
}
