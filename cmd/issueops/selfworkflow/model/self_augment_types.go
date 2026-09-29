package model

import contract "issueops/internal/contract/selfaugment"

const (
	SelfVerificationSummaryKind     = "self_verification_summary"
	SelfVerificationKoreanName      = contract.SelfVerificationKoreanName
	SelfAugmentationKoreanName      = contract.SelfAugmentationKoreanName
	DefaultLoopTargetScoreExclusive = 95.0
)

type SelfAugmentIteration = contract.SelfAugmentIteration
