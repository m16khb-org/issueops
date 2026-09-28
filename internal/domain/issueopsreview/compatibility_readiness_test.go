package issueopsreview

import (
	"reflect"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestCompatibilityReviewMissingPreservesOrderedGates(t *testing.T) {
	if got := CompatibilityReviewMissing(nil); !reflect.DeepEqual(got, []string{"compatibility_review"}) {
		t.Fatalf("missing review=%v", got)
	}
	review := &reviewcontract.CompatibilityReview{Blockers: []string{"unresolved"}}
	want := []string{"backward_compatibility", "side_effects", "rollback_plan", "compatibility_verification", "compatibility_blockers", "compatibility_approval"}
	if got := CompatibilityReviewMissing(review); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
	review.BackwardCompatibility = []string{"unchanged"}
	review.SideEffects = []string{"none"}
	review.RollbackPlan = "revert"
	review.Verification = []string{"go test"}
	review.Blockers = nil
	review.Approved = true
	if got := CompatibilityReviewMissing(review); len(got) != 0 {
		t.Fatalf("approved review missing=%v", got)
	}
}
