package summary

import (
	augmentcontract "issueops/internal/contract/selfaugment"

	app "issueops/internal/application/selfverify"
)

func SummarizeSelfAugment(result augmentcontract.Result) augmentcontract.Summary {
	return app.SummarizeSelfVerification(result, defaultLoopTargetScoreExclusive)
}
