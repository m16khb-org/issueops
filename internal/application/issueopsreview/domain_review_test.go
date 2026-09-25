package issueopsreview

import (
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRecordDomainReviewUsesOneLockedWrite(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.IssueOpsRecord{ID: "io-domain", Phase: model.IssueOpsPhaseGrill}
	reads, writes := 0, 0
	store := reviewport.ReviewMutationStore{
		WithLock:         func(_, _ string, fn func() error) error { return fn() },
		Read:             func(_, _ string) (model.IssueOpsRecord, error) { reads++; return record, nil },
		ValidateMutation: func(model.IssueOpsRecord) error { return nil },
		Write:            func(_ string, next model.IssueOpsRecord) (model.IssueOpsRecord, error) { writes++; return next, nil },
		Now:              func() string { return at },
	}
	out, err := RecordDomainReview(store, "state", record.ID, model.IssueOpsDomainReviewRequest{
		ModelFit: " fits ", Terminology: []string{" aggregate ", " "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reads != 2 || writes != 1 || out.DomainReview == nil || out.DomainReview.ModelFit != "fits" ||
		len(out.DomainReview.Terminology) != 1 || out.UpdatedAt != at {
		t.Fatalf("domain review = %+v, reads=%d writes=%d", out, reads, writes)
	}
}
