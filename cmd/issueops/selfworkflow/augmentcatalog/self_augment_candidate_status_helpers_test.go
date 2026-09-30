package augmentcatalog

import domain "issueops/internal/domain/selfaugment"

func MarkSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	domain.MarkSatisfiedCandidate(candidate, signals)
}

func SelfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return domain.CandidateScore(candidate)
}
