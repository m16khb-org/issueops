package historycompare

import (
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

func CompareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	return historyService().Compare(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate SelfAugmentStateSnapshot) SelfAugmentCompareResult {
	return app.CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, statestore.StateDir())
}

func NewSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) SelfAugmentCompareResult {
	return domain.NewCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct, statestore.StateDir())
}
