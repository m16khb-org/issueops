package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestDesignReviewMissingMapsPersistedReview(t *testing.T) {
	record := model.IssueOpsRecord{DesignReview: &model.IssueOpsDesignReview{Approved: false}}
	want := []string{"problem_summary", "proposed_design", "design_verification", "design_approval"}
	if got := DesignReviewMissing(record); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
}
