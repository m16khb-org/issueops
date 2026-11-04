package issueopsreview

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRecordSchemaEvidenceObservesBeforeLockAndWritesOnce(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.Record{ID: "io-schema", Phase: model.PhaseImplement, WorktreePath: "/repo.worktrees/run"}
	order := []string{}
	store := reviewport.EvidenceReviewStore{
		Read:             func(_, _ string) (model.Record, error) { order = append(order, "read"); return record, nil },
		Fingerprint:      func(model.Record) string { order = append(order, "observe"); return "fingerprint" },
		WithLock:         func(_, _ string, fn func() error) error { order = append(order, "lock"); return fn() },
		ValidateMutation: func(model.Record) error { order = append(order, "authority"); return nil },
		Write: func(_ string, next model.Record) (model.Record, error) {
			order = append(order, "write")
			return next, nil
		},
		Now: func() string { return at },
	}
	out, err := RecordSchemaEvidence(store, "state", record.ID, model.SchemaEvidenceRequest{
		Measurements: []string{" orders rows=1 "}, Sources: []string{" psql "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ","); got != "read,observe,lock,read,authority,write" {
		t.Fatalf("effect order = %q", got)
	}
	if out.SchemaEvidence == nil || out.SchemaEvidence.ReviewedFingerprint != "fingerprint" ||
		len(out.SchemaEvidence.Measurements) != 1 || out.UpdatedAt != at {
		t.Fatalf("recorded schema evidence = %+v", out)
	}
}

func TestRecordSchemaEvidenceRejectsChangedBaseWithoutWrite(t *testing.T) {
	observed := model.Record{ID: "io-schema", Phase: model.PhaseImplement, Repo: "/repo"}
	current := observed
	observed.BranchPrepare = &model.BranchPrepare{BaseBranch: "main", BaseSHA: strings.Repeat("a", 40)}
	current.BranchPrepare = &model.BranchPrepare{BaseBranch: "main", BaseSHA: strings.Repeat("b", 40)}
	reads, writes := 0, 0
	store := reviewport.EvidenceReviewStore{
		Read: func(_, _ string) (model.Record, error) {
			reads++
			if reads == 1 {
				return observed, nil
			}
			return current, nil
		},
		Fingerprint:      func(model.Record) string { return "fingerprint" },
		WithLock:         func(_, _ string, fn func() error) error { return fn() },
		ValidateMutation: func(model.Record) error { return nil },
		Write:            func(_ string, next model.Record) (model.Record, error) { writes++; return next, nil },
		Now:              func() string { return "2026-09-25T00:00:00Z" },
	}
	_, err := RecordSchemaEvidence(store, "state", observed.ID, model.SchemaEvidenceRequest{
		Measurements: []string{"orders rows=1"}, Sources: []string{"psql"},
	})
	if err == nil || !strings.Contains(err.Error(), "changed its worktree or base") || writes != 0 {
		t.Fatalf("changed base = %v, writes=%d", err, writes)
	}
}
