package model

import (
	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
)

type SelfAugmentResult = selfaugmentcontract.SelfAugmentResult

type SelfVerifyLLMEvalResult = selfaugmentcontract.SelfVerifyLLMEvalResult

type SelfAugmentSummary = selfaugmentcontract.SelfAugmentSummary

type SelfVerificationContract = selfverifycontract.SelfVerificationContract

type SelfVerificationGoalScore = selfverifycontract.SelfVerificationGoalScore

type SelfVerificationCoverage = selfverifycontract.SelfVerificationCoverage

type SelfVerificationFailureCluster = selfverifycontract.SelfVerificationFailureCluster

type SelfAugmentSlowStep = selfaugmentcontract.SelfAugmentSlowStep

type SelfAugmentStepDurationStat = selfaugmentcontract.SelfAugmentStepDurationStat

type SelfVerificationGoalDefinition = selfverifycontract.SelfVerificationGoalDefinition

type SelfVerificationCoverageDefinition = selfverifycontract.SelfVerificationCoverageDefinition

type StepResult = selfverifycontract.StepResult
