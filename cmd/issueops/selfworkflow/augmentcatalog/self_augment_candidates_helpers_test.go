package augmentcatalog

import app "issueops/internal/application/selfaugment"

func SelfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return app.Candidates(signals)
}
