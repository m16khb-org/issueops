package historycompare

import (
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func CompareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (augmentcontract.SelfAugmentCompareResult, error) {
	return historyService().Compare(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate augmentcontract.SelfAugmentStateSnapshot) augmentcontract.SelfAugmentCompareResult {
	return app.CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, statestore.StateDir())
}
