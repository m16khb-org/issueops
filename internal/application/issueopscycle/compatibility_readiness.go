package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

// CompatibilityReviewMissing 함수는 기록의 호환성 리뷰 증거에서 빠진 항목 키를 돌려준다. 판정은 review domain이 한다.
func CompatibilityReviewMissing(record model.Record) []string {
	return reviewdomain.CompatibilityReviewMissing(record.CompatibilityReview)
}
