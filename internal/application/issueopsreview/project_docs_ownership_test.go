package issueopsreview

import (
	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
	"strings"
	"testing"
)

func TestProjectDocsUseCaseRejectsUnchangedDocument(t *testing.T) {
	record := model.Record{ID: "io-docs", Phase: model.PhaseImplement, WorktreePath: "/repo"}
	writes := 0
	store := reviewport.ProjectDocsReviewStore{
		EvidenceReviewStore: reviewport.EvidenceReviewStore{
			Read:             func(string, string) (model.Record, error) { return record, nil },
			Fingerprint:      func(model.Record) string { return "fingerprint" },
			WithLock:         func(_, _ string, fn func() error) error { return fn() },
			ValidateMutation: func(model.Record) error { return nil },
			Now:              func() string { return "2026-09-30T00:00:00Z" },
			Write:            func(_ string, r model.Record) (model.Record, error) { writes++; return r, nil },
		},
		ChangedPaths: func(model.Record) []string { return []string{"other.md"} },
		Root:         func(model.Record) string { return "/repo" },
		RelativePath: func(_ string, path string) string { return path },
		FileExists:   func(string, string) bool { return true },
	}
	_, err := RecordProjectDocsReview(store, "state", record.ID, model.ProjectDocsReviewRequest{Verdict: "updated", Docs: []string{"AGENTS.md"}, Evidence: []string{"claimed update"}})
	if err == nil || !strings.Contains(err.Error(), "not in the current change set") || writes != 0 {
		t.Fatalf("unchanged document must be rejected without writes: err=%v writes=%d", err, writes)
	}
}
