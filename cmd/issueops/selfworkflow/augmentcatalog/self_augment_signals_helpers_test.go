package augmentcatalog

import (
	domain "issueops/internal/domain/selfaugment"
)

func SelectedCandidateID(candidate *SelfAugmentCandidate) string {
	return domain.SelectedCandidateID(candidate)
}
