package issueops_test

import (
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func TestPhaseDecisionsPreserveCurrentOrder(t *testing.T) {
	if !issueopsdomain.KnownIssueOpsPhase(issueopscontract.PhaseImplement) || issueopsdomain.PhaseRank(issueopscontract.PhaseImplement) != 5 {
		t.Fatal("implement phase identity drift")
	}
	if issueopsdomain.KnownIssueOpsPhase("unknown") || issueopsdomain.PhaseRank("unknown") != 0 {
		t.Fatal("unknown phase accepted")
	}
}
