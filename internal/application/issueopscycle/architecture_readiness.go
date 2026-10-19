package issueopscycle

import (
	"errors"

	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

func ArchitectureReviewMissing(record model.IssueOpsRecord) []string {
	return reviewdomain.ArchitectureReviewMissing(record.ArchitectureReview)
}

// ArchitectureReviewStaleMissing는 실행이 만들어진 뒤 봉인된 아키텍처 문서가 승인한
// 문서와 같은지 본다. 봉인 파일은 digest가 필요할 때만 읽는다. 읽기는 resolver가,
// 판정은 review domain이 소유한다.
func ArchitectureReviewStaleMissing(record model.IssueOpsRecord, checkBinding bool, resolveDigest func(model.IssueOpsRecord) (string, error)) []string {
	review := record.ArchitectureReview
	if !checkBinding || record.Execution == nil || review == nil || !review.Approved || review.ReviewerContext == cycledomain.ParentReviewPattern {
		return nil
	}
	if resolveDigest == nil {
		resolveDigest = func(model.IssueOpsRecord) (string, error) {
			return "", errors.New("architecture digest resolver is unavailable")
		}
	}
	digest, err := resolveDigest(record)
	if reviewdomain.ArchitectureReviewStale(review, digest, err) {
		return []string{"architecture_review_stale"}
	}
	return nil
}
