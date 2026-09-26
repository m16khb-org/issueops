package summary

import domain "issueops/internal/domain/selfaugment"

func StepDurationStatByLabel(stats []SelfAugmentStepDurationStat) map[string]SelfAugmentStepDurationStat {
	return domain.StepDurationStatByLabel(stats)
}
func MaxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return domain.MaxSlowStepDurationByLabel(steps)
}
func BuildStepDurationStats(durationsByLabel map[string][]int64) []SelfAugmentStepDurationStat {
	return domain.BuildStepDurationStats(durationsByLabel)
}
func StepDurationStatsForCompare(summary SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return domain.StepDurationStatsForCompare(summary)
}
