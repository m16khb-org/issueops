package augmentcatalog

import (
	contract "issueops/internal/contract/selfaugment"
)

const (
	SelfAugmentCandidateStatusOpen      = contract.CandidateStatusOpen
	SelfAugmentCandidateStatusSatisfied = contract.CandidateStatusSatisfied
)

type SelfAugmentCandidate = contract.SelfAugmentCandidate
type SelfAugmentGoal = contract.SelfAugmentGoal
type SelfAugmentInfluence = contract.SelfAugmentInfluence
type SelfAugmentRepoSignals = contract.SelfAugmentRepoSignals
