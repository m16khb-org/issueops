package selfverify

import contract "issueops/internal/contract/selfverify"

// CandidateIDsByStatus returns the IDs of candidates whose Status equals status, in input order.
// The result is never nil.
func CandidateIDsByStatus(candidates []contract.SelfVerificationCandidate, status string) []string {
	ids := []string{}
	for _, candidate := range candidates {
		if candidate.Status == status {
			ids = append(ids, candidate.ID)
		}
	}
	return ids
}

// SelectedCandidate returns a copy of the open candidate with the highest Score, breaking ties by the
// lowest Priority value. It returns nil when no candidate is open.
func SelectedCandidate(candidates []contract.SelfVerificationCandidate) *contract.SelfVerificationCandidate {
	var selected *contract.SelfVerificationCandidate
	for _, candidate := range candidates {
		if candidate.Status != contract.CandidateStatusOpen {
			continue
		}
		if selected == nil || candidate.Score > selected.Score || (candidate.Score == selected.Score && candidate.Priority < selected.Priority) {
			copyCandidate := candidate
			selected = &copyCandidate
		}
	}
	return selected
}

// SelectedCandidateID returns the ID of candidate, or "none" when candidate is nil.
func SelectedCandidateID(candidate *contract.SelfVerificationCandidate) string {
	if candidate == nil {
		return "none"
	}
	return candidate.ID
}
