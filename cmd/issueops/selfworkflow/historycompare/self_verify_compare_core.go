package historycompare

import (
	"fmt"
	"strings"

	"issueops/cmd/issueops/selfworkflow/stateio"
	domain "issueops/internal/domain/selfaugment"
)

func CompareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	result := NewSelfAugmentCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct)
	if strings.TrimSpace(baselineKey) == "" {
		return result, fmt.Errorf("baseline-key is required")
	}
	if strings.TrimSpace(candidateKey) == "" {
		return result, fmt.Errorf("candidate-key is required")
	}
	if maxElapsedRegressionPct < 0 {
		return result, fmt.Errorf("max elapsed regression pct must be non-negative")
	}
	baseline, err := stateio.ReadSelfAugmentStateSnapshot(baselineKey)
	if err != nil {
		return result, fmt.Errorf("read baseline summary: %w", err)
	}
	candidate, err := stateio.ReadSelfAugmentStateSnapshot(candidateKey)
	if err != nil {
		return result, fmt.Errorf("read candidate summary: %w", err)
	}
	return CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate), nil
}

func CompareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate SelfAugmentStateSnapshot) SelfAugmentCompareResult {
	stateio.NormalizeSelfAugmentSnapshotFailureCause(&baseline)
	stateio.NormalizeSelfAugmentSnapshotFailureCause(&candidate)
	return domain.CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, StateDir())
}

func NewSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) SelfAugmentCompareResult {
	return domain.NewCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct, StateDir())
}
