package issueopsreview

import (
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRecordProjectDocsReviewObservesPathsBeforeLockAndWritesOnce(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.IssueOpsRecord{ID: "io-docs", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/repo.worktrees/run"}
	order := []string{}
	store := reviewport.ProjectDocsReviewStore{
		EvidenceReviewStore: reviewport.EvidenceReviewStore{
			Read:             func(_, _ string) (model.IssueOpsRecord, error) { order = append(order, "read"); return record, nil },
			Fingerprint:      func(model.IssueOpsRecord) string { order = append(order, "observe"); return "fingerprint" },
			WithLock:         func(_, _ string, fn func() error) error { order = append(order, "lock"); return fn() },
			ValidateMutation: func(model.IssueOpsRecord) error { order = append(order, "authority"); return nil },
			Write: func(_ string, next model.IssueOpsRecord) (model.IssueOpsRecord, error) {
				order = append(order, "write")
				return next, nil
			},
			Now: func() string { return at },
		},
		NormalizeDocs: func(_ model.IssueOpsRecord, docs []string) ([]string, error) {
			order = append(order, "docs")
			return docs, nil
		},
		NormalizeReviewedDocs: func(_ model.IssueOpsRecord, docs []string) ([]string, error) {
			order = append(order, "reviewed")
			return docs, nil
		},
	}
	out, err := RecordProjectDocsReview(store, "state", record.ID, model.IssueOpsProjectDocsReviewRequest{
		Verdict: " NO-CHANGE ", ReviewedDocs: []string{"AGENTS.md"}, Evidence: []string{" read "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "read,observe,docs,reviewed,lock,read,authority,write" {
		t.Fatalf("effect order = %q", got)
	}
	if out.ProjectDocsReview == nil || out.ProjectDocsReview.Verdict != "no-change" ||
		out.ProjectDocsReview.ReviewedFingerprint != "fingerprint" || out.UpdatedAt != at {
		t.Fatalf("recorded project docs review = %+v", out)
	}
}

func TestRecordProjectDocsReviewDefersPathErrorUntilAfterAuthority(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-docs", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/repo.worktrees/run"}
	pathErr := errors.New("path outside worktree")
	authorityErr := errors.New("no current holder")
	writes := 0
	store := reviewport.ProjectDocsReviewStore{
		EvidenceReviewStore: reviewport.EvidenceReviewStore{
			Read:             func(_, _ string) (model.IssueOpsRecord, error) { return record, nil },
			Fingerprint:      func(model.IssueOpsRecord) string { return "fingerprint" },
			WithLock:         func(_, _ string, fn func() error) error { return fn() },
			ValidateMutation: func(model.IssueOpsRecord) error { return authorityErr },
			Write:            func(_ string, next model.IssueOpsRecord) (model.IssueOpsRecord, error) { writes++; return next, nil },
			Now:              func() string { return "2026-09-25T00:00:00Z" },
		},
		NormalizeDocs:         func(_ model.IssueOpsRecord, docs []string) ([]string, error) { return nil, pathErr },
		NormalizeReviewedDocs: func(_ model.IssueOpsRecord, docs []string) ([]string, error) { return docs, nil },
	}
	_, err := RecordProjectDocsReview(store, "state", record.ID, model.IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Docs: []string{"../outside"}, Evidence: []string{"updated"},
	})
	if !errors.Is(err, authorityErr) || writes != 0 {
		t.Fatalf("authority must precede deferred path error: %v, writes=%d", err, writes)
	}
}
