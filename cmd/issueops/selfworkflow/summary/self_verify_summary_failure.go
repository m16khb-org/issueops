package summary

import domain "issueops/internal/domain/selfverify"

func ClassifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return domain.ClassifyFailure(summary.FailedSteps, summary.TotalRuns, projectRuns(result))
}

func SelfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return domain.FailureClusters(projectRuns(result))
}
