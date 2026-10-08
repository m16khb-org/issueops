package issueops

import (
	"context"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	issueopsstatusdomain "issueops/internal/domain/issueopsstatus"
)

// IssueOpsStatus must backfill phases missing from a PARTIAL persisted ledger
// (e.g. a multi-phase forward jump that stamped only its endpoints), not only
// when the ledger is entirely empty — while preserving the real persisted
// entries rather than overwriting them with derived ones.
func TestIssueOpsStatusBackfillsPartialLedger(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	rec, err := startIssueOpsFixture(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "1-partial"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Persist a partial ledger: problem + plan stamped, grill absent.
	rec.Phase = issueops.IssueOpsPhasePlan
	rec.PhaseLedger = issueops.IssueOpsPhaseLedger{
		issueops.IssueOpsPhaseProblem: {Phase: issueops.IssueOpsPhaseProblem, EnteredAt: "2026-06-29T00:00:00Z", CompletedAt: "2026-06-29T00:01:00Z"},
		issueops.IssueOpsPhasePlan:    {Phase: issueops.IssueOpsPhasePlan, EnteredAt: "2026-06-29T00:02:00Z"},
	}
	if _, err := touchAndWriteIssueOps(context.Background(), stateRoot, rec); err != nil {
		t.Fatalf("write: %v", err)
	}

	status, err := readIssueOpsStatusForTest(stateRoot, rec.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, phase := range issueops.IssueOpsPhases {
		if _, ok := status.PhaseLedger[phase]; !ok {
			t.Fatalf("partial ledger must be backfilled; missing %s: %#v", phase, status.PhaseLedger)
		}
	}
	// The real persisted entry must win over the derived one.
	if got := status.PhaseLedger[issueops.IssueOpsPhaseProblem].CompletedAt; got != "2026-06-29T00:01:00Z" {
		t.Fatalf("persisted problem entry must be preserved, got CompletedAt=%q", got)
	}
}

// A forward transition that re-completes a previously-regressed phase must clear
// the stale-regression note so status no longer shows the phase as stale forever.
func TestStampForwardTransitionClearsStaleNote(t *testing.T) {
	t.Parallel()

	ledger := issueopsdomain.MarkLedgerStale(issueops.IssueOpsPhaseLedger{}, "stale: design-review regression (second-system effect)", issueops.IssueOpsPhasePlan)
	if len(ledger[issueops.IssueOpsPhasePlan].Notes) == 0 {
		t.Fatal("precondition: plan must carry a stale note before re-completion")
	}

	ledger = issueopsdomain.StampForwardTransition(ledger, issueops.IssueOpsPhasePlan, issueops.IssueOpsPhaseCompatibilityReview, "2026-06-30T00:00:00Z", issueopsstatusdomain.ArtifactKeys(issueops.IssueOpsPhasePlan))
	plan := ledger[issueops.IssueOpsPhasePlan]
	if plan.CompletedAt == "" {
		t.Fatalf("plan must be marked complete after the forward transition, got %#v", plan)
	}
	for _, n := range plan.Notes {
		if strings.HasPrefix(n, "stale:") {
			t.Fatalf("stale note must be cleared on legitimate re-completion, got notes=%v", plan.Notes)
		}
	}
}
