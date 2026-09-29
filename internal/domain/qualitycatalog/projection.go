package qualitycatalog

func ApplyPlanStatus(catalog []Candidate, plan []Candidate) []Candidate {
	candidates := append([]Candidate{}, catalog...)
	byID := make(map[string]Candidate, len(plan))
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
