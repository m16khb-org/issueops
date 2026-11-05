package issueops

import (
	reviewcontract "issueops/internal/contract/issueopsreview"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestMissingPreconditionsAcceptsReviewedParent(t *testing.T) {
	parent := model.Record{
		Phase:                model.PhaseImplement,
		Branch:               "123-parent",
		ArchitectureReview:   &reviewcontract.ArchitectureReview{Approved: true},
		CompatibilityReview:  &reviewcontract.CompatibilityReview{Approved: true},
		DevilsAdvocateReview: &model.DevilsAdvocateReview{Verdict: "pass", RecordedAt: "2026-07-07T00:00:00Z"},
	}
	if missing := ChildStartMissingPreconditions(parent, model.ChildStartRequest{Branch: "123-child"}); len(missing) != 0 {
		t.Fatalf("reviewed parent should satisfy delegation gate, got %#v", missing)
	}
}

func TestChildMutationRetainsFinishAuthority(t *testing.T) {
	record := model.Record{CleanupAttempt: &model.CleanupAttempt{Operation: "finish", Token: "pending", StartedAt: "2026-09-29T00:00:00Z"}}
	if err := ValidateChildMutation(record); err == nil {
		t.Fatal("child mutation bypassed finish attempt")
	}
	record.CleanupAttempt = nil
	if err := ValidateChildMutation(record); err != nil {
		t.Fatal(err)
	}
}
