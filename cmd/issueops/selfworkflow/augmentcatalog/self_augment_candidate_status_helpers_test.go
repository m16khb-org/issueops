package augmentcatalog

import domain "issueops/internal/domain/selfaugment"

func MarkSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	domain.MarkSatisfiedCandidate(candidate, signals)
}
