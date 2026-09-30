package selfverify

import contract "issueops/internal/contract/selfverify"

func CandidateIDsByStatus(candidates []contract.SelfVerificationCandidate, status string) []string {
	ids := []string{}
	for _, candidate := range candidates {
		if candidate.Status == status {
			ids = append(ids, candidate.ID)
		}
	}
	return ids
}

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

func SelectedCandidateID(candidate *contract.SelfVerificationCandidate) string {
	if candidate == nil {
		return "none"
	}
	return candidate.ID
}
