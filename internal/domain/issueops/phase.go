package issueops

import issueopscontract "issueops/internal/contract/issueops"

func KnownIssueOpsPhase(phase issueopscontract.IssueOpsPhase) bool {
	return IssueOpsPhaseRank(phase) != 0
}

func IssueOpsPhaseRank(phase issueopscontract.IssueOpsPhase) int {
	for index, known := range issueopscontract.IssueOpsPhases {
		if phase == known {
			return index + 1
		}
	}
	return 0
}
