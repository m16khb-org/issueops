package issueopsreview

import (
	"errors"
	"reflect"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

func TestArchitectureReviewMissingPreservesOrderedApprovalGates(t *testing.T) {
	if got := ArchitectureReviewMissing(nil); !reflect.DeepEqual(got, []string{"architecture_review"}) {
		t.Fatalf("missing review=%v", got)
	}
	review := &reviewcontract.ArchitectureReview{
		ProblemSummary: "problem", ProposedArchitecture: "architecture", Verification: []string{"go test"},
		Approved: true, OpenQuestions: []string{"unresolved"},
	}
	want := []string{"architecture_review_evidence", "architecture_open_questions", "refactor_plan", "alternatives", "risks"}
	if got := ArchitectureReviewMissing(review); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
	review.Verification = []string{"architecture review checked risks"}
	review.OpenQuestions = nil
	review.RefactorPlan = "plan"
	review.Alternatives = []string{"other"}
	review.Risks = []string{"risk"}
	if got := ArchitectureReviewMissing(review); len(got) != 0 {
		t.Fatalf("approved review missing=%v", got)
	}
}

func TestArchitectureReviewStaleComparesApprovedDigestWithCurrent(t *testing.T) {
	approved := func(digest, context string) *reviewcontract.ArchitectureReview {
		return &reviewcontract.ArchitectureReview{Approved: true, ReviewedArchitectureDigest: digest, ReviewerContext: context}
	}
	for _, tc := range []struct {
		name    string
		review  *reviewcontract.ArchitectureReview
		current string
		err     error
		want    bool
	}{
		{"no review is reported by the review gate instead", nil, "abc", nil, false},
		{"unapproved review is reported by the approval gate instead", &reviewcontract.ArchitectureReview{ReviewedArchitectureDigest: "abc"}, "def", nil, false},
		{"same digest ignoring case", approved("AbC", "subagent"), "abc", nil, false},
		{"edited document", approved("abc", "subagent"), "def", nil, true},
		{"approval recorded without a digest", approved("", "inline"), "abc", nil, true},
		{"document unreadable", approved("abc", "subagent"), "", errors.New("gone"), true},
		{"inherited parent approval is exempt", approved("", issueopsdomain.ParentReviewPattern), "def", errors.New("gone"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ArchitectureReviewStale(tc.review, tc.current, tc.err); got != tc.want {
				t.Fatalf("stale=%v, want %v", got, tc.want)
			}
		})
	}
}
