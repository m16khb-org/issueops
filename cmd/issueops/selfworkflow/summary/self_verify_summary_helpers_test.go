package summary

import (
	augmentcontract "issueops/internal/contract/selfaugment"

	app "issueops/internal/application/selfverify"
)

func SummarizeSelfAugment(result augmentcontract.SelfAugmentResult) augmentcontract.SelfAugmentSummary {
	return app.SummarizeSelfVerification(result, defaultLoopTargetScoreExclusive)
}
