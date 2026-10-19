package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

func DevilsAdvocateDigestExempt(review reviewcontract.DevilsAdvocateReview) bool {
	return review.ReviewerPattern == issueopsdomain.ParentReviewPattern
}

func DevilsAdvocatePlanDigestRequired(review *reviewcontract.DevilsAdvocateReview, checkPlanBinding, hasLinkedPlan bool) bool {
	return review != nil && strings.TrimSpace(review.RecordedAt) != "" &&
		checkPlanBinding && hasLinkedPlan && !DevilsAdvocateDigestExempt(*review) &&
		strings.TrimSpace(review.ReviewedPlanDigest) != ""
}

func DevilsAdvocateReviewMissing(review *reviewcontract.DevilsAdvocateReview, checkPlanBinding, hasLinkedPlan bool, currentDigest string, digestErr error) []string {
	if review == nil || strings.TrimSpace(review.RecordedAt) == "" {
		return []string{"devils_advocate_review"}
	}
	missing := []string{}
	if (review.Verdict == "stop" || review.Verdict == "revise") && !review.Waived {
		missing = append(missing, "devils_advocate_review")
	}
	if checkPlanBinding && hasLinkedPlan && !DevilsAdvocateDigestExempt(*review) {
		if strings.TrimSpace(review.ReviewedPlanDigest) == "" || digestErr != nil || !strings.EqualFold(currentDigest, review.ReviewedPlanDigest) {
			missing = append(missing, "devils_advocate_review_stale")
		}
	}
	return missing
}
