package summary

import (
	"issueops/cmd/issueops/commandstep"
	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
)

const defaultLoopTargetScoreExclusive = 95.0

type SelfAugmentResult = augmentcontract.SelfAugmentResult
type SelfAugmentSlowStep = augmentcontract.SelfAugmentSlowStep
type SelfAugmentStepDurationStat = augmentcontract.SelfAugmentStepDurationStat
type SelfAugmentSummary = augmentcontract.SelfAugmentSummary
type SelfVerificationContract = verifycontract.SelfVerificationContract
type SelfVerificationCoverage = verifycontract.SelfVerificationCoverage
type SelfVerificationCoverageDefinition = verifycontract.SelfVerificationCoverageDefinition
type SelfVerificationFailureCluster = verifycontract.SelfVerificationFailureCluster
type SelfVerificationGoalDefinition = verifycontract.SelfVerificationGoalDefinition
type SelfVerificationGoalScore = verifycontract.SelfVerificationGoalScore
type StepResult = commandstep.StepResult
