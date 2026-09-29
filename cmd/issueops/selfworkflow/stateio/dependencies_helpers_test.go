package stateio

import (
	"issueops/cmd/issueops/selfworkflow/augmentcatalog"
	"issueops/cmd/issueops/selfworkflow/model"
	augmentcontract "issueops/internal/contract/selfaugment"
)

const (
	selfAugmentCandidateStatusOpen      = augmentcatalog.SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = augmentcatalog.SelfAugmentCandidateStatusSatisfied
	selfAugmentationPlanKind            = model.SelfAugmentationPlanKind
	selfVerificationSummaryKind         = model.SelfVerificationSummaryKind
)

type SelfAugmentCandidate = model.SelfAugmentCandidate
type SelfAugmentPlanResult = model.SelfAugmentPlanResult
type SelfAugmentPlanStateSnapshot = augmentcontract.SelfAugmentPlanStateSnapshot
type SelfAugmentPromoteResult = augmentcontract.SelfAugmentPromoteResult
type SelfAugmentResult = model.SelfAugmentResult
type SelfAugmentStateCheckpoint = augmentcontract.SelfAugmentStateCheckpoint
type SelfAugmentStateSnapshot = augmentcontract.SelfAugmentStateSnapshot
