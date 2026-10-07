package issueopsreview

import (
	"errors"
	"strings"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestValidateReviewRequiresEvidenceAndKeepsInjectedTime(t *testing.T) {
	const recordedAt = "2026-09-24T00:00:00Z"
	request := reviewcontract.DevilsAdvocateReviewRequest{
		Verdict: " Pass ", ReviewerContext: " Inline ", Findings: []string{"  attacked gate 3  ", "attacked gate 3", " "},
	}
	review, err := ValidateReview(request, recordedAt)
	if err != nil {
		t.Fatal(err)
	}
	if review.Verdict != "pass" || review.ReviewerContext != "inline" || len(review.Findings) != 1 || review.Findings[0] != "attacked gate 3" || review.RecordedAt != recordedAt {
		t.Fatalf("normalized review = %+v", review)
	}
	request.Findings = nil
	if _, err := ValidateReview(request, recordedAt); err == nil || !strings.Contains(err.Error(), "pass verdict requires") {
		t.Fatalf("pass without findings accepted: %v", err)
	}
}

func TestApplyReviewEnforcesReviseRoundCapAndPreservesHistory(t *testing.T) {
	fourth := &reviewcontract.DevilsAdvocateReview{
		Verdict: "revise", RecordedAt: "fourth", History: []reviewcontract.DevilsAdvocateRound{
			{Verdict: "revise", RecordedAt: "first"}, {Verdict: "revise", RecordedAt: "second"}, {Verdict: "revise", RecordedAt: "third"},
		},
	}
	fifth, err := ApplyReview(fourth, reviewcontract.DevilsAdvocateReview{Verdict: "revise", RecordedAt: "fifth"})
	if err != nil || len(fifth.History) != 4 {
		t.Fatalf("fifth revise should be allowed: %+v %v", fifth, err)
	}
	_, err = ApplyReview(&fifth, reviewcontract.DevilsAdvocateReview{Verdict: "revise"})
	capErr, ok := errors.AsType[*ReviseRoundCapError](err)
	if !ok || capErr.Count != 5 {
		t.Fatalf("sixth revise should hit cap: %T %v", err, err)
	}
	next, err := ApplyReview(&fifth, reviewcontract.DevilsAdvocateReview{Verdict: "pass", RecordedAt: "sixth"})
	if err != nil || len(next.History) != 5 || next.History[4].RecordedAt != "fifth" {
		t.Fatalf("history lost: %+v %v", next, err)
	}
}
