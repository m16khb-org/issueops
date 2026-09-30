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
		ChangedPaths: func(model.IssueOpsRecord) []string {
			t.Fatal("no-change review must not enumerate changed paths")
			return nil
		},
		Root:         func(model.IssueOpsRecord) string { return "/repo.worktrees/run" },
		RelativePath: func(_ string, path string) string { order = append(order, "reviewed"); return path },
		FileExists:   func(string, string) bool { order = append(order, "file"); return true },
	}
	out, err := RecordProjectDocsReview(store, "state", record.ID, model.IssueOpsProjectDocsReviewRequest{
		Verdict: " NO-CHANGE ", ReviewedDocs: []string{"AGENTS.md"}, Evidence: []string{" read "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "read,observe,reviewed,file,lock,read,authority,write" {
		t.Fatalf("effect order = %q", got)
	}
	if out.ProjectDocsReview == nil || out.ProjectDocsReview.Verdict != "no-change" ||
		out.ProjectDocsReview.ReviewedFingerprint != "fingerprint" || out.UpdatedAt != at {
		t.Fatalf("recorded project docs review = %+v", out)
	}
}

func TestRecordProjectDocsReviewDefersPathErrorUntilAfterAuthority(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-docs", Phase: model.IssueOpsPhaseImplement, WorktreePath: "/repo.worktrees/run"}
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
		ChangedPaths: func(model.IssueOpsRecord) []string { return nil },
		Root:         func(model.IssueOpsRecord) string { return "/repo.worktrees/run" },
		RelativePath: func(string, string) string { return "" },
		FileExists:   func(string, string) bool { t.Fatal("invalid path must not touch filesystem"); return false },
	}
	_, err := RecordProjectDocsReview(store, "state", record.ID, model.IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Docs: []string{"../outside"}, Evidence: []string{"updated"},
	})
	if !errors.Is(err, authorityErr) || writes != 0 {
		t.Fatalf("authority must precede deferred path error: %v, writes=%d", err, writes)
	}
}
