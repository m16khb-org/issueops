package issueopscycle

import (
	"strings"

	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

// DevilsAdvocateReviewMissing observes the linked plan only when its digest is
// needed. The resolver owns filesystem access; the review domain owns the gate.
func DevilsAdvocateReviewMissing(record model.Record, checkPlanBinding bool, resolveDigest func(model.Record) (string, error)) []string {
	hasLinkedPlan := strings.TrimSpace(record.PlanPath) != ""
	var digest string
	var err error
	if reviewdomain.DevilsAdvocatePlanDigestRequired(record.DevilsAdvocateReview, checkPlanBinding, hasLinkedPlan) {
		digest, err = resolveDigest(record)
	}
	return reviewdomain.DevilsAdvocateReviewMissing(record.DevilsAdvocateReview, checkPlanBinding, hasLinkedPlan, digest, err)
}
