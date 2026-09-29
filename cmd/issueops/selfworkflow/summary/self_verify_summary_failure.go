package summary

import app "issueops/internal/application/selfverify"

func ClassifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return app.ClassifySelfVerificationFailure(result, summary)
}
func SelfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return app.SelfVerificationFailureClusters(result)
}
