package issueopsreview

import (
	"errors"
	"reflect"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestDevilsAdvocateReviewMissingPreservesVerdictAndPlanBinding(t *testing.T) {
	if got := DevilsAdvocateReviewMissing(nil, true, true, "", nil); !reflect.DeepEqual(got, []string{"devils_advocate_review"}) {
		t.Fatalf("missing review=%v", got)
	}
	review := &reviewcontract.DevilsAdvocateReview{Verdict: "stop", RecordedAt: "now", ReviewedPlanDigest: "AbC"}
	if got := DevilsAdvocateReviewMissing(review, true, true, "abc", nil); !reflect.DeepEqual(got, []string{"devils_advocate_review"}) {
		t.Fatalf("unwaived stop=%v", got)
	}
	review.Waived = true
	if got := DevilsAdvocateReviewMissing(review, true, true, "abc", nil); len(got) != 0 {
		t.Fatalf("waived matching review=%v", got)
	}
	if got := DevilsAdvocateReviewMissing(review, true, true, "", errors.New("unreadable")); !reflect.DeepEqual(got, []string{"devils_advocate_review_stale"}) {
		t.Fatalf("unreadable plan=%v", got)
	}
	review.ReviewedPlanDigest = ""
	if got := DevilsAdvocateReviewMissing(review, true, true, "", nil); !reflect.DeepEqual(got, []string{"devils_advocate_review_stale"}) {
		t.Fatalf("undigested review=%v", got)
	}
	review.ReviewerPattern = ParentReviewPattern
	if got := DevilsAdvocateReviewMissing(review, true, true, "", nil); len(got) != 0 {
		t.Fatalf("inherited parent review=%v", got)
	}
}

func TestDevilsAdvocatePlanDigestRequiredOnlyWhenObserved(t *testing.T) {
	review := &reviewcontract.DevilsAdvocateReview{RecordedAt: "now", ReviewedPlanDigest: "abc"}
	if !DevilsAdvocatePlanDigestRequired(review, true, true) {
		t.Fatal("current linked plan must be observed")
	}
	for _, tc := range []struct {
		name    string
		review  *reviewcontract.DevilsAdvocateReview
		binding bool
		plan    bool
	}{
		{"missing review", nil, true, true},
		{"blank timestamp", &reviewcontract.DevilsAdvocateReview{}, true, true},
		{"clean stage", review, false, true},
		{"no plan", review, true, false},
		{"undigested review", &reviewcontract.DevilsAdvocateReview{RecordedAt: "now"}, true, true},
		{"inherited parent", &reviewcontract.DevilsAdvocateReview{RecordedAt: "now", ReviewedPlanDigest: "abc", ReviewerPattern: ParentReviewPattern}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if DevilsAdvocatePlanDigestRequired(tc.review, tc.binding, tc.plan) {
				t.Fatal("digest observation must be skipped")
			}
		})
	}
}

func TestDevilsAdvocateStagedPlanBoundPreservesPreImplementGate(t *testing.T) {
	review := &reviewcontract.DevilsAdvocateReview{ReviewedPlanDigest: "AbC"}
	if !DevilsAdvocateStagedPlanBound(nil, true, "other") || !DevilsAdvocateStagedPlanBound(review, false, "other") {
		t.Fatal("unreviewed or post-implement plan must not be checked")
	}
	if !DevilsAdvocateStagedPlanBound(review, true, "abc") {
		t.Fatal("matching digest must pass without case sensitivity")
	}
	if DevilsAdvocateStagedPlanBound(review, true, "other") {
		t.Fatal("mismatched digest must fail")
	}
	review.ReviewerPattern = ParentReviewPattern
	if !DevilsAdvocateStagedPlanBound(review, true, "other") {
		t.Fatal("delegated child inherits parent review")
	}
}
