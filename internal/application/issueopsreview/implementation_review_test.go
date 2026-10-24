package issueopsreview

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRecordImplementationReviewObservesBeforeLockAndWritesOnce(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.IssueOpsRecord{ID: "io-review", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/repo.worktrees/run"}
	order := []string{}
	store := reviewport.EvidenceReviewStore{
		Read:             func(_, _ string) (model.IssueOpsRecord, error) { order = append(order, "read"); return record, nil },
		Fingerprint:      func(model.IssueOpsRecord) string { order = append(order, "observe"); return "fingerprint" },
		WithLock:         func(_, _ string, fn func() error) error { order = append(order, "lock"); return fn() },
		ValidateMutation: func(model.IssueOpsRecord) error { order = append(order, "authority"); return nil },
		Write: func(_ string, next model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			order = append(order, "write")
			return next, nil
		},
		Now: func() string { return at },
	}
	out, err := RecordImplementationReview(store, "state", record.ID, model.IssueOpsImplementationReviewRequest{
		Verdict: "PASS", Findings: []string{" finding "}, Evidence: []string{" test "}, ReviewerHost: " CODEX ", ReviewerContext: " Inline ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "read,observe,lock,read,authority,write" {
		t.Fatalf("effect order = %q", got)
	}
	if out.ImplementationReview == nil || out.ImplementationReview.Verdict != "pass" ||
		out.ImplementationReview.ReviewedFingerprint != "fingerprint" || out.ImplementationReview.ReviewerHost != "codex" ||
		out.ImplementationReview.ReviewerContext != "inline" ||
		out.UpdatedAt != at {
		t.Fatalf("recorded review = %+v", out)
	}
}

// Plan reviews record how they ran; implementation reviews could not, so
// agents told to review inline wrote it into finding text. An unknown context
// is refused before any read, like the plan reviews.
func TestRecordImplementationReviewRefusesUnknownReviewerContext(t *testing.T) {
	store := reviewport.EvidenceReviewStore{Read: func(_, _ string) (model.IssueOpsRecord, error) {
		t.Fatal("refusal must come before the read")
		return model.IssueOpsRecord{}, nil
	}}
	_, err := RecordImplementationReview(store, "state", "io-review", model.IssueOpsImplementationReviewRequest{
		Verdict: "pass", Findings: []string{"f"}, Evidence: []string{"e"}, ReviewerContext: "pair",
	})
	if err == nil || !strings.Contains(err.Error(), "subagent or inline") {
		t.Fatalf("err = %v", err)
	}
}

func TestRecordImplementationReviewRejectsChangedObservationWithoutWrite(t *testing.T) {
	observed := model.IssueOpsRecord{ID: "io-review", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/repo.worktrees/old"}
	current := observed
	current.WorktreePath = "/repo.worktrees/new"
	reads, writes := 0, 0
	store := reviewport.EvidenceReviewStore{
		Read: func(_, _ string) (model.IssueOpsRecord, error) {
			reads++
			if reads == 1 {
				return observed, nil
			}
			return current, nil
		},
		Fingerprint:      func(model.IssueOpsRecord) string { return "fingerprint" },
		WithLock:         func(_, _ string, fn func() error) error { return fn() },
		ValidateMutation: func(model.IssueOpsRecord) error { return nil },
		Write:            func(_ string, next model.IssueOpsRecord) (model.IssueOpsRecord, error) { writes++; return next, nil },
		Now:              func() string { return "2026-09-25T00:00:00Z" },
	}
	_, err := RecordImplementationReview(store, "state", observed.ID, model.IssueOpsImplementationReviewRequest{
		Verdict: "pass", Findings: []string{"finding"}, Evidence: []string{"test"},
	})
	if err == nil || !strings.Contains(err.Error(), "changed its worktree or base") || writes != 0 {
		t.Fatalf("changed observation = %v, writes=%d", err, writes)
	}
}
