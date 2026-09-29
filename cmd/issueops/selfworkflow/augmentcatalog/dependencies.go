package augmentcatalog

import (
	"issueops/cmd/issueops/selfworkflow/model"
	contract "issueops/internal/contract/selfaugment"
)

const (
	SelfAugmentCandidateStatusOpen      = contract.CandidateStatusOpen
	SelfAugmentCandidateStatusSatisfied = contract.CandidateStatusSatisfied
)

type SelfAugmentCandidate = model.SelfAugmentCandidate
type SelfAugmentGoal = model.SelfAugmentGoal
type SelfAugmentInfluence = model.SelfAugmentInfluence
type SelfAugmentRepoSignals = model.SelfAugmentRepoSignals
