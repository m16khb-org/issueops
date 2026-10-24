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

// DevilsAdvocateStagedPlanStale reports that the staged plan changed after the
// plan review bound its digest. Before link-plan the implement readiness has
// no linked plan to compare, so a plan revised after the review reached
// plan.handoff unnoticed. Reviews without a digest or inherited from the parent
// are not judged here.
func DevilsAdvocateStagedPlanStale(review *reviewcontract.DevilsAdvocateReview, stagedPlan string) bool {
	if !DevilsAdvocatePlanDigestRequired(review, true, true) {
		return false
	}
	digest, err := StagedReviewPlanDigest(stagedPlan)
	return err != nil || !strings.EqualFold(digest, review.ReviewedPlanDigest)
}
