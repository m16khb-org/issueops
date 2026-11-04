package selfaugment

import (
	"sort"

	contract "issueops/internal/contract/selfaugment"
)

const stepBudgetMinRegressionMS int64 = 25

// CompareSlowestStepRegressions reports labels whose slowest observed duration grew by more than
// maxRegressionPct from baseline to candidate. Labels absent from baseline or with a non-positive
// baseline are ignored; the result is ordered by descending growth, then label.
func CompareSlowestStepRegressions(baseline, candidate []contract.SlowStep, maxRegressionPct float64) []contract.SlowStepRegression {
	baselineByLabel := MaxSlowStepDurationByLabel(baseline)
	candidateByLabel := MaxSlowStepDurationByLabel(candidate)
	regressions := []contract.SlowStepRegression{}
	for label, candidateDuration := range candidateByLabel {
		baselineDuration, ok := baselineByLabel[label]
		if !ok || baselineDuration <= 0 {
			continue
		}
		delta := candidateDuration - baselineDuration
		if delta <= 0 {
			continue
		}
		deltaPct := float64(delta) * 100 / float64(baselineDuration)
		if deltaPct <= maxRegressionPct {
			continue
		}
		regressions = append(regressions, contract.SlowStepRegression{
			Label:               label,
			BaselineDurationMS:  baselineDuration,
			CandidateDurationMS: candidateDuration,
			DeltaMS:             delta,
			DeltaPct:            deltaPct,
		})
	}
	sort.Slice(regressions, func(i, j int) bool {
		if regressions[i].DeltaPct != regressions[j].DeltaPct {
			return regressions[i].DeltaPct > regressions[j].DeltaPct
		}
		return regressions[i].Label < regressions[j].Label
	})
	return regressions
}

// CompareStepBudgetRegressions reports labels whose p95 duration grew by more than maxRegressionPct
// and at least 25 ms from baseline to candidate. Labels without a positive baseline p95 are
// ignored; the result is ordered by descending growth, then label.
func CompareStepBudgetRegressions(baseline, candidate []contract.StepDurationStat, maxRegressionPct float64) []contract.StepBudgetRegression {
	baselineByLabel := StepDurationStatByLabel(baseline)
	candidateByLabel := StepDurationStatByLabel(candidate)
	regressions := []contract.StepBudgetRegression{}
	for label, candidateStat := range candidateByLabel {
		baselineStat, ok := baselineByLabel[label]
		if !ok || baselineStat.P95DurationMS <= 0 {
			continue
		}
		delta := candidateStat.P95DurationMS - baselineStat.P95DurationMS
		if delta <= 0 {
			continue
		}
		if delta < stepBudgetMinRegressionMS {
			continue
		}
		deltaPct := float64(delta) * 100 / float64(baselineStat.P95DurationMS)
		if deltaPct <= maxRegressionPct {
			continue
		}
		regressions = append(regressions, contract.StepBudgetRegression{
			Label:               label,
			Metric:              "p95_duration_ms",
			BaselineDurationMS:  baselineStat.P95DurationMS,
			CandidateDurationMS: candidateStat.P95DurationMS,
			DeltaMS:             delta,
			DeltaPct:            deltaPct,
			BaselineCount:       baselineStat.Count,
			CandidateCount:      candidateStat.Count,
		})
	}
	sort.Slice(regressions, func(i, j int) bool {
		if regressions[i].DeltaPct != regressions[j].DeltaPct {
			return regressions[i].DeltaPct > regressions[j].DeltaPct
		}
		return regressions[i].Label < regressions[j].Label
	})
	return regressions
}

// MissingStrings returns the sorted entries of want that are absent from have.
func MissingStrings(want, have []string) []string {
	haveSet := map[string]bool{}
	for _, item := range have {
		haveSet[item] = true
	}
	missing := []string{}
	for _, item := range want {
		if !haveSet[item] {
			missing = append(missing, item)
		}
	}
	sort.Strings(missing)
	return missing
}
