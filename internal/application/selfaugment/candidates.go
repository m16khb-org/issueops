package selfaugment

import (
	contract "issueops/internal/contract/selfaugment"
	"issueops/internal/domain/qualitycatalog"
	domain "issueops/internal/domain/selfaugment"
)

// Candidates returns the improvement candidates for signals: the quality
// catalog's candidate specs combined by the selfaugment domain. Slices in the
// result are copies, so callers may modify them.
func Candidates(signals contract.RepoSignals) []contract.Candidate {
	specs := qualitycatalog.CandidateSpecs()
	qualityCandidates := make([]contract.Candidate, 0, len(specs))
	for _, spec := range specs {
		qualityCandidates = append(qualityCandidates, contract.Candidate{
			ID: spec.ID, Title: spec.Title, Category: spec.Category,
			VerificationKind: spec.VerificationKind,
			Impact:           spec.Impact, Feasibility: spec.Feasibility, Novelty: spec.Novelty, Risk: spec.Risk,
			WhyNow:       append([]string{}, spec.WhyNow...),
			ExpectedGain: append([]string{}, spec.ExpectedGain...),
			VerifyWith:   append([]string{}, spec.VerifyWith...),
		})
	}
	return domain.Candidates(signals, qualityCandidates)
}
