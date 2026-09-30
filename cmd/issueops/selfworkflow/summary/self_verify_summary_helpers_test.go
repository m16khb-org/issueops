package summary

import app "issueops/internal/application/selfverify"

func SummarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return app.SummarizeSelfVerification(result, defaultLoopTargetScoreExclusive)
}
func SummarizeSelfVerification(result SelfAugmentResult, target float64) SelfAugmentSummary {
	return app.SummarizeSelfVerification(result, target)
}
