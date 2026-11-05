package issueops

import issueopscontract "issueops/internal/contract/issueops"

// KnownIssueOpsPhase reports whether phase is one of the declared IssueOps
// phases.
func KnownIssueOpsPhase(phase issueopscontract.Phase) bool {
	return PhaseRank(phase) != 0
}

// PhaseRank returns the 1-based position of phase in the declared phase
// order, or 0 for an unknown phase. Callers compare ranks to decide whether a
// cycle has reached or passed a phase.
func PhaseRank(phase issueopscontract.Phase) int {
	for index, known := range issueopscontract.Phases {
		if phase == known {
			return index + 1
		}
	}
	return 0
}
