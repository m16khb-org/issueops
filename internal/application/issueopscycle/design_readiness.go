package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

func DesignReviewMissing(record model.IssueOpsRecord) []string {
	return reviewdomain.DesignReviewMissing(record.DesignReview)
}
