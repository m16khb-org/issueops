package quality

import (
	catalog "issueops/internal/contract/qualitycatalog"
	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/qualitycatalog"
)

func CandidatesForPlan(plan []contract.SelfAugmentCandidate) []catalog.Candidate {
	projected := make([]catalog.Candidate, len(plan))
	for i, candidate := range plan {
		projected[i] = catalog.Candidate{ID: candidate.ID, Status: candidate.Status, Score: candidate.Score}
	}
	return domain.ApplyPlanStatus(domain.Candidates(), projected)
}
