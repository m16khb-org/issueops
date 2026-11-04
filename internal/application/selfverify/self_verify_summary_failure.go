package selfverify

import (
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

func ClassifySelfVerificationFailure(result augment.Result, summary augment.Summary) (string, string, []verify.SelfVerificationFailureCluster) {
	return domain.ClassifyFailure(summary.FailedSteps, summary.TotalRuns, projectRuns(result))
}
