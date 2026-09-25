package issueops

import (
	"fmt"

	issueopscontract "issueops/internal/contract/issueops"
)

func ValidatePhaseProgression(from, to issueopscontract.IssueOpsPhase) error {
	if from == issueopscontract.IssueOpsPhaseDone {
		return fmt.Errorf("cannot leave done phase")
	}
	if IssueOpsPhaseRank(to) < IssueOpsPhaseRank(from) {
		return fmt.Errorf("cannot move issueops phase backward from %s to %s", from, to)
	}
	return nil
}
