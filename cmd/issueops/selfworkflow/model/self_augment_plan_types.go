package model

import contract "issueops/internal/contract/selfaugment"

const (
	SelfAugmentationPlanKind   = contract.SelfAugmentationPlanKind
	SelfAugmentationLessonKind = contract.SelfAugmentationLessonKind
)

type SelfAugmentPlanRequest = contract.SelfAugmentPlanRequest
type SelfAugmentPlanResult = contract.SelfAugmentPlanResult
type SelfAugmentInfluence = contract.SelfAugmentInfluence
type SelfAugmentLessonRequest = contract.SelfAugmentLessonRequest
type SelfAugmentLessonResult = contract.SelfAugmentLessonResult
type SelfAugmentLessonStateSnapshot = contract.SelfAugmentLessonStateSnapshot
type SelfAugmentPlanStateSnapshot = contract.SelfAugmentPlanStateSnapshot
type SelfAugmentGoal = contract.SelfAugmentGoal
type SelfAugmentCandidate = contract.SelfAugmentCandidate
type SelfAugmentRepoSignals = contract.SelfAugmentRepoSignals
