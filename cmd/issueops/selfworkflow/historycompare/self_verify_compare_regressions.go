package historycompare

import domain "issueops/internal/domain/selfaugment"

func CompareSlowestStepRegressions(baseline, candidate []SelfAugmentSlowStep, maxRegressionPct float64) []SelfAugmentSlowStepRegression {
	return domain.CompareSlowestStepRegressions(baseline, candidate, maxRegressionPct)
}
func CompareStepBudgetRegressions(baseline, candidate []SelfAugmentStepDurationStat, maxRegressionPct float64) []SelfAugmentStepBudgetRegression {
	return domain.CompareStepBudgetRegressions(baseline, candidate, maxRegressionPct)
}
func MissingStrings(want, have []string) []string { return domain.MissingStrings(want, have) }
