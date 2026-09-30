package issueopsreview

import reviewcontract "issueops/internal/contract/issueopsreview"

func InheritedParentReview(parentID, now string) *reviewcontract.DevilsAdvocateReview {
	return &reviewcontract.DevilsAdvocateReview{Verdict: "pass", Waived: true, WaiverRationale: "delegated:" + parentID + " parent DA verdict pass", ReviewerPattern: ParentReviewPattern, RecordedAt: now, IssueReflectedAt: ""}
}
