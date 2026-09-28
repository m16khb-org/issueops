package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

func CompatibilityReviewMissing(record model.IssueOpsRecord) []string {
	return reviewdomain.CompatibilityReviewMissing(record.CompatibilityReview)
}
