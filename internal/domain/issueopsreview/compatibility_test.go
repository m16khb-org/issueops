package issueopsreview

import (
	"strings"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestValidateCompatibilityReviewEvidenceAndBlockers(t *testing.T) {
	request := reviewcontract.CompatibilityReviewRequest{
		BackwardCompatibility: []string{" additive field ", "additive field"},
		SideEffects:           []string{"none"}, Verification: []string{"go test ./..."},
		RollbackPlan: " git revert ", Approved: true,
	}
	review, err := ValidateCompatibilityReview(request, "2026-09-24T00:00:00Z")
	if err != nil || !review.Approved || len(review.BackwardCompatibility) != 1 || review.RollbackPlan != "git revert" || review.ReviewedAt != "2026-09-24T00:00:00Z" {
		t.Fatalf("compatibility review = %+v %v", review, err)
	}
	request.Blockers = []string{"schema drift"}
	if _, err := ValidateCompatibilityReview(request, "2026-09-24T00:00:00Z"); err == nil || !strings.Contains(err.Error(), "must not have blockers") {
		t.Fatalf("approved review with blocker accepted: %v", err)
	}
	request.Approved = false
	request.BackwardCompatibility = nil
	if _, err := ValidateCompatibilityReview(request, "2026-09-24T00:00:00Z"); err == nil || !strings.Contains(err.Error(), "backward_compatibility") {
		t.Fatalf("missing evidence accepted: %v", err)
	}
}
