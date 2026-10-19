package issueopscycle

import (
	"errors"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	cycledomain "issueops/internal/domain/issueops"
)

func TestArchitectureReviewMissingMapsPersistedReview(t *testing.T) {
	record := model.IssueOpsRecord{ArchitectureReview: &reviewcontract.ArchitectureReview{Approved: false}}
	want := []string{"problem_summary", "proposed_architecture", "architecture_verification", "architecture_approval"}
	if got := ArchitectureReviewMissing(record); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
}

func TestArchitectureReviewStaleMissingReadsSealedDigestOnlyWhenNeeded(t *testing.T) {
	executing := func(review *reviewcontract.ArchitectureReview) model.IssueOpsRecord {
		return model.IssueOpsRecord{ArchitectureReview: review, Execution: &model.Execution{}}
	}
	approved := &reviewcontract.ArchitectureReview{Approved: true, ReviewerContext: "subagent", ReviewedArchitectureDigest: "abc"}
	reads := 0
	resolve := func(digest string, err error) func(model.IssueOpsRecord) (string, error) {
		return func(model.IssueOpsRecord) (string, error) { reads++; return digest, err }
	}
	for _, tc := range []struct {
		name    string
		record  model.IssueOpsRecord
		check   bool
		resolve func(model.IssueOpsRecord) (string, error)
		want    []string
		reads   int
	}{
		{"sealed digest matches", executing(approved), true, resolve("abc", nil), nil, 1},
		{"sealed architecture edited", executing(approved), true, resolve("def", nil), []string{"architecture_review_stale"}, 1},
		{"sealed architecture missing", executing(approved), true, resolve("", errors.New("gone")), []string{"architecture_review_stale"}, 1},
		{"resolver unavailable counts as stale", executing(approved), true, nil, []string{"architecture_review_stale"}, 0},
		{"binding check disabled", executing(approved), false, resolve("def", nil), nil, 0},
		{"before execution there is nothing sealed", model.IssueOpsRecord{ArchitectureReview: approved}, true, resolve("def", nil), nil, 0},
		{"inherited approval is exempt", executing(&reviewcontract.ArchitectureReview{Approved: true, ReviewerContext: cycledomain.ParentReviewPattern}), true, resolve("def", nil), nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads = 0
			got := ArchitectureReviewStaleMissing(tc.record, tc.check, tc.resolve)
			if !reflect.DeepEqual(got, tc.want) || reads != tc.reads {
				t.Fatalf("missing=%v reads=%d, want %v reads=%d", got, reads, tc.want, tc.reads)
			}
		})
	}
}
