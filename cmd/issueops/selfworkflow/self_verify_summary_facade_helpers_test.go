package selfworkflow

import (
	verifyapp "issueops/internal/application/selfverify"
	augmentdomain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
)

func summarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return verifyapp.SummarizeSelfVerification(result, 95)
}

func summarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	return verifyapp.SummarizeSelfVerification(result, targetScore)
}

func selfVerificationContract() SelfVerificationContract {
	return verifydomain.ContractValue()
}

func selfVerificationGoalDefinitions() []selfVerificationGoalDefinition {
	return verifydomain.GoalDefinitions()
}

func selfVerificationCoverageDefinitions() []selfVerificationCoverageDefinition {
	return verifydomain.CoverageDefinitions()
}

func selfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	return verifydomain.CoverageForLabels(stepLabels)
}

func scoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	return verifyapp.MapGoalScores(result, targetScore)
}

func classifySelfVerificationFailure(result SelfAugmentResult, summaryValue SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return verifyapp.ClassifySelfVerificationFailure(result, summaryValue)
}

func selfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return verifyapp.SelfVerificationFailureClusters(result)
}

func stepDurationStatByLabel(stats []SelfAugmentStepDurationStat) map[string]SelfAugmentStepDurationStat {
	return augmentdomain.StepDurationStatByLabel(stats)
}

func maxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return augmentdomain.MaxSlowStepDurationByLabel(steps)
}

func buildStepDurationStats(durationsByLabel map[string][]int64) []SelfAugmentStepDurationStat {
	return augmentdomain.BuildStepDurationStats(durationsByLabel)
}

func stepDurationStatsForCompare(summaryValue SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return augmentdomain.StepDurationStatsForCompare(summaryValue)
}
