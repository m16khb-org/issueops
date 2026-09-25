package issueopsreview

import (
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRegressPersistsOneAuditedTransition(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.IssueOpsRecord{
		ID: "io-regress", Phase: model.IssueOpsPhasePlan,
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{Verdict: "stop", IssueReflectedAt: at},
		DesignReview:         &model.IssueOpsDesignReview{Approved: true},
	}
	writes := 0
	store := reviewport.RegressStore{
		Read:           func(_, _ string) (model.IssueOpsRecord, error) { return record, nil },
		ActiveChildren: func(_ string, _ model.IssueOpsRecord) ([]string, error) { return nil, nil },
		Now:            func() string { return at },
		TouchWrite: func(_ string, updated model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			return updated, nil
		},
	}
	out, err := Regress(store, "state", record.ID, "  scope changed  ")
	if err != nil {
		t.Fatal(err)
	}
	if writes != 1 || out.Phase != model.IssueOpsPhaseGrill || len(out.RegressEvents) != 1 ||
		len(out.Decisions) != 1 || out.DevilsAdvocateReview != nil || out.DesignReview.Approved {
		t.Fatalf("regression did not persist one complete transition: writes=%d record=%+v", writes, out)
	}
	if out.RegressEvents[0].Reason != "scope changed" || out.UpdatedAt != at {
		t.Fatalf("regression reason/time = %+v, updated_at=%s", out.RegressEvents[0], out.UpdatedAt)
	}
}

func TestRegressRejectsActiveChildrenWithoutWriting(t *testing.T) {
	const at = "2026-09-25T00:00:00Z"
	record := model.IssueOpsRecord{
		ID: "io-regress", Phase: model.IssueOpsPhasePlan,
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{Verdict: "stop", IssueReflectedAt: at},
	}
	writes := 0
	store := reviewport.RegressStore{
		Read:           func(_, _ string) (model.IssueOpsRecord, error) { return record, nil },
		ActiveChildren: func(_ string, _ model.IssueOpsRecord) ([]string, error) { return []string{"io-child"}, nil },
		Now:            func() string { return at },
		TouchWrite: func(_ string, updated model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			writes++
			return updated, nil
		},
	}
	if _, err := Regress(store, "state", record.ID, "scope changed"); err == nil || writes != 0 {
		t.Fatalf("active child regression = %v, writes=%d", err, writes)
	}
}
