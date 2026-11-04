package selfverify

import (
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

// ClassifySelfVerificationFailure returns the failure class, its reason and the failure clusters
// for a result whose summary reports failed steps.
func ClassifySelfVerificationFailure(result augment.Result, summary augment.Summary) (string, string, []verify.SelfVerificationFailureCluster) {
	return domain.ClassifyFailure(summary.FailedSteps, summary.TotalRuns, projectRuns(result))
}
