package issueopsreview

import (
	reviewcontract "issueops/internal/contract/issueopsreview"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRegressPersistsOneAuditedTransition(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.Record{
		ID: "io-regress", Phase: model.PhasePlan,
		DevilsAdvocateReview: &model.DevilsAdvocateReview{Verdict: "stop", IssueReflectedAt: at},
		ArchitectureReview:   &reviewcontract.ArchitectureReview{Approved: true},
	}
	writes := 0
	store := reviewport.RegressStore{
		Read:           func(_, _ string) (model.Record, error) { return record, nil },
		ActiveChildren: func(_ string, _ model.Record) ([]string, error) { return nil, nil },
		Now:            func() string { return at },
		TouchWrite: func(_ string, updated model.Record) (model.Record, error) {
			writes++
			return updated, nil
		},
	}
	out, err := Regress(store, "state", record.ID, "  scope changed  ")
	if err != nil {
		t.Fatal(err)
	}
	if writes != 1 || out.Phase != model.PhaseGrill || len(out.RegressEvents) != 1 ||
		len(out.Decisions) != 1 || out.DevilsAdvocateReview != nil || out.ArchitectureReview.Approved {
		t.Fatalf("regression did not persist one complete transition: writes=%d record=%+v", writes, out)
	}
	if out.RegressEvents[0].Reason != "scope changed" || out.UpdatedAt != at {
		t.Fatalf("regression reason/time = %+v, updated_at=%s", out.RegressEvents[0], out.UpdatedAt)
	}
}

func TestRegressRejectsActiveChildrenWithoutWriting(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.Record{
		ID: "io-regress", Phase: model.PhasePlan,
		DevilsAdvocateReview: &model.DevilsAdvocateReview{Verdict: "stop", IssueReflectedAt: at},
	}
	writes := 0
	store := reviewport.RegressStore{
		Read:           func(_, _ string) (model.Record, error) { return record, nil },
		ActiveChildren: func(_ string, _ model.Record) ([]string, error) { return []string{"io-child"}, nil },
		Now:            func() string { return at },
		TouchWrite: func(_ string, updated model.Record) (model.Record, error) {
			writes++
			return updated, nil
		},
	}
	if _, err := Regress(store, "state", record.ID, "scope changed"); err == nil || writes != 0 {
		t.Fatalf("active child regression = %v, writes=%d", err, writes)
	}
}
