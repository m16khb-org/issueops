package issueopsreview

import (
	"strings"
	"testing"
)

func TestFeedbackRequiresIssueUpdate(t *testing.T) {
	if !FeedbackRequiresIssueUpdate(" Contract_Change ", "") {
		t.Fatal("unresolved contract change must require an issue update")
	}
	if FeedbackRequiresIssueUpdate("contract_change", "2026-09-24T00:00:00Z") || FeedbackRequiresIssueUpdate("defect", "") {
		t.Fatal("resolved or non-contract feedback must not require an issue update")
	}
}

func TestFeedbackPhaseAfterAdd(t *testing.T) {
	if phase, err := FeedbackPhaseAfterAdd("implement", false); err != nil || phase != "implement" {
		t.Fatalf("ordinary feedback phase = %q, %v", phase, err)
	}
	if phase, err := FeedbackPhaseAfterAdd("ai-slop-clean", true); err != nil || phase != "feedback" {
		t.Fatalf("feedback after clean phase = %q, %v", phase, err)
	}
	if _, err := FeedbackPhaseAfterAdd("done", true); err == nil || !strings.Contains(err.Error(), "cannot add feedback after done") {
		t.Fatalf("done feedback must be rejected: %v", err)
	}
}

func TestValidateFeedbackIndex(t *testing.T) {
	for _, index := range []int{-1, 2} {
		if err := ValidateFeedbackIndex(index, 2); err == nil || !strings.Contains(err.Error(), "out of range") {
			t.Fatalf("index %d should be rejected: %v", index, err)
		}
	}
	if err := ValidateFeedbackIndex(1, 2); err != nil {
		t.Fatal(err)
	}
}
