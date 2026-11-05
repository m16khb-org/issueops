package issueops_test

import (
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func TestIssueOpsPhaseOrderingAndClassification(t *testing.T) {
	if !issueopsdomain.KnownIssueOpsPhase(issueopscontract.PhaseProblem) || !issueopsdomain.KnownIssueOpsPhase(issueopscontract.PhaseDone) {
		t.Fatal("expected first and last phases to be known")
	}
	if issueopsdomain.KnownIssueOpsPhase("unknown") {
		t.Fatal("unexpected unknown phase")
	}
	if issueopsdomain.PhaseRank(issueopscontract.PhaseProblem) != 1 || issueopsdomain.PhaseRank(issueopscontract.PhaseDone) != len(issueopscontract.Phases) {
		t.Fatalf("unexpected phase ranks: problem=%d done=%d", issueopsdomain.PhaseRank(issueopscontract.PhaseProblem), issueopsdomain.PhaseRank(issueopscontract.PhaseDone))
	}
	if issueopsdomain.PhaseRank("unknown") != 0 {
		t.Fatal("unknown phase should have rank 0")
	}
	if !issueopsdomain.KnownIssueOpsPhase(issueopscontract.PhaseCompatibilityReview) {
		t.Fatal("compatibility-review phase should be known")
	}
	if issueopsdomain.PhaseRank(issueopscontract.PhaseCompatibilityReview) <= issueopsdomain.PhaseRank(issueopscontract.PhasePlan) ||
		issueopsdomain.PhaseRank(issueopscontract.PhaseCompatibilityReview) >= issueopsdomain.PhaseRank(issueopscontract.PhaseImplement) {
		t.Fatalf("compatibility-review should sit between plan and implement, got rank %d", issueopsdomain.PhaseRank(issueopscontract.PhaseCompatibilityReview))
	}
}
