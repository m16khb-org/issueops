package issueopsreview

import (
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

func InheritedParentReview(parentID, now string) *reviewcontract.DevilsAdvocateReview {
	return &reviewcontract.DevilsAdvocateReview{Verdict: "pass", Waived: true, WaiverRationale: "delegated:" + parentID + " parent DA verdict pass", ReviewerPattern: issueopsdomain.ParentReviewPattern, RecordedAt: now, IssueReflectedAt: ""}
}
