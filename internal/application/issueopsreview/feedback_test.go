package issueopsreview

import (
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestFeedbackUseCasesKeepAuthorityBeforeMutationAndOneWrite(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.IssueOpsRecord{ID: "io-feedback", Phase: model.IssueOpsPhaseAISlopClean, AISlopCleanAt: at}
	reads, writes := 0, 0
	store := reviewport.FeedbackStore{
		WithLock:         func(_, _ string, fn func() error) error { return fn() },
		Read:             func(_, _ string) (model.IssueOpsRecord, error) { reads++; return record, nil },
		ValidateMutation: func(model.IssueOpsRecord) error { return nil },
		Write: func(_ string, next model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			record = next
			return next, nil
		},
		Now: func() string { return at },
	}
	added, err := AddFeedback(store, "state", record.ID, " review ", " contract changed ", " CONTRACT_CHANGE ")
	if err != nil {
		t.Fatal(err)
	}
	if added.Phase != model.IssueOpsPhaseFeedback || len(added.Feedback) != 1 ||
		added.Feedback[0].Classification != "contract_change" || reads != 2 || writes != 1 {
		t.Fatalf("added feedback = %+v, reads=%d writes=%d", added, reads, writes)
	}
	marked, err := MarkContractFeedbackIssueUpdated(store, "state", record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if marked.Feedback[0].IssueUpdatedAt != at || reads != 4 || writes != 2 {
		t.Fatalf("marked feedback = %+v, reads=%d writes=%d", marked, reads, writes)
	}
	resolved, err := ResolveFeedback(store, "state", record.ID, 0, " VALID-DEFECT ")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Feedback[0].Resolution != "valid-defect" || reads != 6 || writes != 3 {
		t.Fatalf("resolved feedback = %+v, reads=%d writes=%d", resolved, reads, writes)
	}
	if _, err := MarkContractFeedbackIssueUpdated(store, "state", record.ID); err == nil ||
		!strings.Contains(err.Error(), "no unresolved contract_change") || writes != 3 {
		t.Fatalf("duplicate mark = %v, writes=%d", err, writes)
	}
}

func TestAddFeedbackRejectsAuthorityBeforeInputAndDoesNotWrite(t *testing.T) {
	authorityErr := errors.New("current holder required")
	reads, writes := 0, 0
	store := reviewport.FeedbackStore{
		WithLock: func(_, _ string, fn func() error) error { return fn() },
		Read: func(_, _ string) (model.IssueOpsRecord, error) {
			reads++
			return model.IssueOpsRecord{ID: "io-feedback", Phase: model.IssueOpsPhaseImplement}, nil
		},
		ValidateMutation: func(model.IssueOpsRecord) error { return authorityErr },
		Write:            func(_ string, next model.IssueOpsRecord) (model.IssueOpsRecord, error) { writes++; return next, nil },
		Now:              func() string { return "2026-09-25T00:00:00Z" },
	}
	_, err := AddFeedback(store, "state", "io-feedback", "", "", "unknown")
	if !errors.Is(err, authorityErr) || reads != 1 || writes != 0 {
		t.Fatalf("authority rejection = %v, reads=%d writes=%d", err, reads, writes)
	}
}
