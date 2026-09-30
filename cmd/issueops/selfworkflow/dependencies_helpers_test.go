package selfworkflow

import (
	"encoding/json"
	"os"

	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
	augmentdomain "issueops/internal/domain/selfaugment"
)

const (
	DefaultLoopTargetScoreExclusive     = 95.0
	SelfAugmentationLessonKind          = augmentcontract.SelfAugmentationLessonKind
	SelfAugmentationKoreanName          = augmentcontract.SelfAugmentationKoreanName
	SelfAugmentationPlanKind            = augmentcontract.SelfAugmentationPlanKind
	SelfVerificationKoreanName          = augmentcontract.SelfVerificationKoreanName
	SelfVerificationSummaryKind         = augmentdomain.SelfVerificationSummaryKind
	SelfAugmentCandidateStatusOpen      = selfAugmentCandidateStatusOpen
	SelfAugmentCandidateStatusSatisfied = selfAugmentCandidateStatusSatisfied
)

type StepResult = verifycontract.StepResult

type SelfAugmentCandidate = augmentcontract.SelfAugmentCandidate
type SelfAugmentGoal = augmentcontract.SelfAugmentGoal
type SelfAugmentInfluence = augmentcontract.SelfAugmentInfluence
type SelfAugmentIteration = augmentcontract.SelfAugmentIteration
type SelfAugmentLessonRequest = augmentcontract.SelfAugmentLessonRequest
type SelfAugmentLessonResult = augmentcontract.SelfAugmentLessonResult
type SelfAugmentLessonStateSnapshot = augmentcontract.SelfAugmentLessonStateSnapshot
type SelfAugmentPlanRequest = augmentcontract.SelfAugmentPlanRequest
type SelfAugmentPlanResult = augmentcontract.SelfAugmentPlanResult
type SelfAugmentRepoSignals = augmentcontract.SelfAugmentRepoSignals
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

type selfVerificationCoverageDefinition = verifycontract.SelfVerificationCoverageDefinition
type selfVerificationGoalDefinition = verifycontract.SelfVerificationGoalDefinition

var IssueOpsRoot = func() string {
	if root := os.Getenv("ISSUEOPS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

var Version = "dev"

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
