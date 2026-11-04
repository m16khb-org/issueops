package historycompare

import (
	statestore "issueops/internal/adapter/outbound/state"
	augmentcontract "issueops/internal/contract/selfaugment"
	augmentdomain "issueops/internal/domain/selfaugment"
)

func CompareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (augmentcontract.CompareResult, error) {
	return historyService().Compare(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate augmentcontract.StateSnapshot) augmentcontract.CompareResult {
	return augmentdomain.CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, statestore.Dir())
}
