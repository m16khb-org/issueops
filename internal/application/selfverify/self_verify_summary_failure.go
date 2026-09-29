package selfverify

import (
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

func ClassifySelfVerificationFailure(result augment.SelfAugmentResult, summary augment.SelfAugmentSummary) (string, string, []verify.SelfVerificationFailureCluster) {
	return domain.ClassifyFailure(summary.FailedSteps, summary.TotalRuns, projectRuns(result))
}

func SelfVerificationFailureClusters(result augment.SelfAugmentResult) []verify.SelfVerificationFailureCluster {
	return domain.FailureClusters(projectRuns(result))
}
