package issueopscycle

import (
	"errors"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestDevilsAdvocateReviewMissingObservesDigestOnlyWhenRequired(t *testing.T) {
	called := 0
	resolve := func(model.IssueOpsRecord) (string, error) {
		called++
		return "abc", nil
	}
	record := model.IssueOpsRecord{PlanPath: "plan.md"}
	if got := DevilsAdvocateReviewMissing(record, true, resolve); !reflect.DeepEqual(got, []string{"devils_advocate_review"}) || called != 0 {
		t.Fatalf("missing review=%v calls=%d", got, called)
	}
	record.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{RecordedAt: "now"}
	if got := DevilsAdvocateReviewMissing(record, true, resolve); !reflect.DeepEqual(got, []string{"devils_advocate_review_stale"}) || called != 0 {
		t.Fatalf("undigested review=%v calls=%d", got, called)
	}
	record.DevilsAdvocateReview.ReviewedPlanDigest = "ABC"
	if got := DevilsAdvocateReviewMissing(record, false, resolve); len(got) != 0 || called != 0 {
		t.Fatalf("clean stage=%v calls=%d", got, called)
	}
	if got := DevilsAdvocateReviewMissing(record, true, resolve); len(got) != 0 || called != 1 {
		t.Fatalf("bound review=%v calls=%d", got, called)
	}
	resolve = func(model.IssueOpsRecord) (string, error) { return "", errors.New("unreadable") }
	if got := DevilsAdvocateReviewMissing(record, true, resolve); !reflect.DeepEqual(got, []string{"devils_advocate_review_stale"}) {
		t.Fatalf("unreadable plan=%v", got)
	}
}
