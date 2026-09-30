package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func CompatibilityReviewMissing(review *reviewcontract.CompatibilityReview) []string {
	if review == nil {
		return []string{"compatibility_review"}
	}
	missing := []string{}
	if len(cleanDesignValues(review.BackwardCompatibility)) == 0 {
		missing = append(missing, "backward_compatibility")
	}
	if len(cleanDesignValues(review.SideEffects)) == 0 {
		missing = append(missing, "side_effects")
	}
	if strings.TrimSpace(review.RollbackPlan) == "" {
		missing = append(missing, "rollback_plan")
	}
	if len(cleanDesignValues(review.Verification)) == 0 {
		missing = append(missing, "compatibility_verification")
	}
	if len(cleanDesignValues(review.Blockers)) > 0 {
		missing = append(missing, "compatibility_blockers")
	}
	if !review.Approved {
		missing = append(missing, "compatibility_approval")
	}
	return missing
}
