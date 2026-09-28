package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

// ParentReviewPattern identifies a delegated child's inherited review. Its own
// plan was not reviewed, so the plan digest gate does not apply.
const ParentReviewPattern = "delegated-parent-review"

func DevilsAdvocateDigestExempt(review reviewcontract.DevilsAdvocateReview) bool {
	return review.ReviewerPattern == ParentReviewPattern
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

// DevilsAdvocateStagedPlanBound requires a matching review only before implement.
// Later owner replacement may reseal a plan edited during implementation.
func DevilsAdvocateStagedPlanBound(review *reviewcontract.DevilsAdvocateReview, beforeImplement bool, stagedDigest string) bool {
	if review == nil || DevilsAdvocateDigestExempt(*review) || !beforeImplement {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(review.ReviewedPlanDigest), stagedDigest)
}
