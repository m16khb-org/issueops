package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestReviewReflectionRequiresFindingsBeforeLinkedIssue(t *testing.T) {
	record := model.IssueOpsRecord{}
	if err := ValidateReviewReflection(record); err == nil || err.Error() != "no devil's-advocate findings to reflect" {
		t.Fatalf("error=%v", err)
	}
	record.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{Findings: []string{"finding"}, IssueReflectedAt: "before"}
	if err := ValidateReviewReflection(record); err == nil || err.Error() != "cannot reflect findings before a linked issue" {
		t.Fatalf("error=%v", err)
	}
	record.IssueURL = "https://example.com/issues/1"
	if err := ValidateReviewReflection(record); err != nil {
		t.Fatal(err)
	}
	got := MarkReviewReflected(record, "after")
	if got.DevilsAdvocateReview.IssueReflectedAt != "after" || got.UpdatedAt != "after" || record.DevilsAdvocateReview.IssueReflectedAt != "before" {
		t.Fatal("stamp mutated input or omitted timestamp")
	}
	if err := ValidateReviewReflectionStamp(model.IssueOpsRecord{}); err == nil {
		t.Fatal("missing review accepted for stamping")
	}
}
