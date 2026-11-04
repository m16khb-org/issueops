package issueops

import issueopscontract "issueops/internal/contract/issueops"

func KnownIssueOpsPhase(phase issueopscontract.Phase) bool {
	return PhaseRank(phase) != 0
}

func PhaseRank(phase issueopscontract.Phase) int {
	for index, known := range issueopscontract.Phases {
		if phase == known {
			return index + 1
		}
	}
	return 0
}
