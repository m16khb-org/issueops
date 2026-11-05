package qualitycatalog

import (
	contract "issueops/internal/contract/qualitycatalog"
)

// ApplyPlanStatus returns a copy of catalog in which each candidate present in
// plan takes that candidate's Status and Score. Candidates absent from plan are
// unchanged.
func ApplyPlanStatus(catalog []contract.Candidate, plan []contract.Candidate) []contract.Candidate {
	candidates := append([]contract.Candidate{}, catalog...)
	byID := make(map[string]contract.Candidate, len(plan))
	for _, candidate := range plan {
		byID[candidate.ID] = candidate
	}
	for i := range candidates {
		if source, ok := byID[candidates[i].ID]; ok {
			candidates[i].Status = source.Status
			candidates[i].Score = source.Score
		}
	}
	return candidates
}
