package stateio

import (
	augmentcontract "issueops/internal/contract/selfaugment"
	augmentdomain "issueops/internal/domain/selfaugment"
)

const (
	selfAugmentCandidateStatusOpen      = augmentcontract.CandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = augmentcontract.CandidateStatusSatisfied
	selfAugmentationPlanKind            = augmentcontract.SelfAugmentationPlanKind
	selfVerificationSummaryKind         = augmentdomain.SelfVerificationSummaryKind
)

type SelfAugmentCandidate = augmentcontract.SelfAugmentCandidate
type SelfAugmentPlanResult = augmentcontract.SelfAugmentPlanResult
type SelfAugmentPlanStateSnapshot = augmentcontract.SelfAugmentPlanStateSnapshot
type SelfAugmentPromoteResult = augmentcontract.SelfAugmentPromoteResult
type SelfAugmentResult = augmentcontract.SelfAugmentResult
type SelfAugmentStateCheckpoint = augmentcontract.SelfAugmentStateCheckpoint
type SelfAugmentStateSnapshot = augmentcontract.SelfAugmentStateSnapshot
