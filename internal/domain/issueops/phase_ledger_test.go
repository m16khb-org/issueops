package issueops

import (
	"slices"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestStampForwardTransitionCompletesPreviousPhaseAndClearsStale(t *testing.T) {
	ledger := issueopscontract.PhaseLedger{
		issueopscontract.PhasePlan: {
			Phase: issueopscontract.PhasePlan, EnteredAt: "earlier",
			Notes: []string{"keep", "stale:regression"},
		},
	}
	got := StampForwardTransition(ledger, issueopscontract.PhasePlan, issueopscontract.PhaseCompatibilityReview, "now", []string{"plan_path"})
	previous := got[issueopscontract.PhasePlan]
	if previous.EnteredAt != "earlier" || previous.CompletedAt != "now" ||
		!slices.Equal(previous.Notes, []string{"keep"}) || len(previous.Artifacts) == 0 || len(previous.Missing) != 0 {
		t.Fatalf("previous phase = %+v", previous)
	}
	entry := got[issueopscontract.PhaseCompatibilityReview]
	if entry.EnteredAt != "now" || entry.CompletedAt != "" {
		t.Fatalf("new phase = %+v", entry)
	}
}

func TestMarkLedgerStaleRetainsEntryAndClearsCompletion(t *testing.T) {
	phase := issueopscontract.PhasePlan
	ledger := issueopscontract.PhaseLedger{phase: {
		Phase: phase, EnteredAt: "entered", CompletedAt: "completed", Notes: []string{"audit"},
	}}
	got := MarkLedgerStale(ledger, "stale: regression", phase)[phase]
	if got.Phase != phase || got.EnteredAt != "entered" || got.CompletedAt != "" ||
		!slices.Equal(got.Notes, []string{"audit", "stale: regression"}) {
		t.Fatalf("stale entry = %+v", got)
	}
}
