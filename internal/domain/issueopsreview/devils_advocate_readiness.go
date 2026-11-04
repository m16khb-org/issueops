package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

func devilsAdvocateDigestExempt(review reviewcontract.DevilsAdvocateReview) bool {
	return review.ReviewerPattern == issueopsdomain.ParentReviewPattern
}

// DevilsAdvocatePlanDigestRequired reports whether the review is bound to a plan
// digest that must still match the linked plan. It is false for a missing or
// unrecorded review, a delegated review inherited from the parent, or a review that
// carries no digest.
func DevilsAdvocatePlanDigestRequired(review *reviewcontract.DevilsAdvocateReview, checkPlanBinding, hasLinkedPlan bool) bool {
	return review != nil && strings.TrimSpace(review.RecordedAt) != "" &&
		checkPlanBinding && hasLinkedPlan && !devilsAdvocateDigestExempt(*review) &&
		strings.TrimSpace(review.ReviewedPlanDigest) != ""
}

// DevilsAdvocateReviewMissing returns the gate keys a devil's-advocate review still
// lacks: devils_advocate_review for a missing, stop, or unwaived revise verdict, and
// devils_advocate_review_stale when the reviewed plan digest no longer matches
// currentDigest.
func DevilsAdvocateReviewMissing(review *reviewcontract.DevilsAdvocateReview, checkPlanBinding, hasLinkedPlan bool, currentDigest string, digestErr error) []string {
	if review == nil || strings.TrimSpace(review.RecordedAt) == "" {
		return []string{"devils_advocate_review"}
	}
	missing := []string{}
	if (review.Verdict == "stop" || review.Verdict == "revise") && !review.Waived {
		missing = append(missing, "devils_advocate_review")
	}
	if checkPlanBinding && hasLinkedPlan && !devilsAdvocateDigestExempt(*review) {
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
