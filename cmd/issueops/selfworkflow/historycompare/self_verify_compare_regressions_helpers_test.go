package historycompare

import domain "issueops/internal/domain/selfaugment"

func CompareStepBudgetRegressions(baseline, candidate []SelfAugmentStepDurationStat, maxRegressionPct float64) []SelfAugmentStepBudgetRegression {
	return domain.CompareStepBudgetRegressions(baseline, candidate, maxRegressionPct)
}
