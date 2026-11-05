package issueopsreview

import (
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

// InheritedParentReview builds the waived pass review a delegated child inherits from
// its parent cycle, recorded at now and marked with the parent review pattern so the
// plan digest gate skips it.
func InheritedParentReview(parentID, now string) *reviewcontract.DevilsAdvocateReview {
	return &reviewcontract.DevilsAdvocateReview{Verdict: "pass", Waived: true, WaiverRationale: "delegated:" + parentID + " parent DA verdict pass", ReviewerPattern: issueopsdomain.ParentReviewPattern, RecordedAt: now, IssueReflectedAt: ""}
}
