package selfaugment

import (
	"sort"

	contract "issueops/internal/contract/selfaugment"
)

func StepDurationStatByLabel(stats []contract.SelfAugmentStepDurationStat) map[string]contract.SelfAugmentStepDurationStat {
	out := map[string]contract.SelfAugmentStepDurationStat{}
	for _, stat := range stats {
		if stat.Label == "" {
			continue
		}
		out[stat.Label] = stat
	}
	return out
}

func MaxSlowStepDurationByLabel(steps []contract.SelfAugmentSlowStep) map[string]int64 {
	out := map[string]int64{}
	for _, step := range steps {
		if step.Label == "" {
			continue
		}
		if step.DurationMS > out[step.Label] {
			out[step.Label] = step.DurationMS
		}
	}
	return out
}

func BuildStepDurationStats(durationsByLabel map[string][]int64) []contract.SelfAugmentStepDurationStat {
	stats := []contract.SelfAugmentStepDurationStat{}
	for label, durations := range durationsByLabel {
		if label == "" || len(durations) == 0 {
			continue
		}
		sortedDurations := append([]int64{}, durations...)
		sort.Slice(sortedDurations, func(i, j int) bool { return sortedDurations[i] < sortedDurations[j] })
		var sum int64
		for _, duration := range sortedDurations {
			sum += duration
		}
		p95Index := (95*len(sortedDurations) + 99) / 100
		if p95Index < 1 {
			p95Index = 1
		}
		stats = append(stats, contract.SelfAugmentStepDurationStat{
			Label:             label,
			Count:             len(sortedDurations),
			MinDurationMS:     sortedDurations[0],
			MaxDurationMS:     sortedDurations[len(sortedDurations)-1],
			AverageDurationMS: float64(sum) / float64(len(sortedDurations)),
			P95DurationMS:     sortedDurations[p95Index-1],
		})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Label < stats[j].Label })
	return stats
}

func StepDurationStatsForCompare(summary contract.SelfAugmentSummary) []contract.SelfAugmentStepDurationStat {
	if len(summary.StepDurationStats) > 0 {
		return append([]contract.SelfAugmentStepDurationStat{}, summary.StepDurationStats...)
	}
	durationsByLabel := map[string][]int64{}
	for _, step := range summary.SlowestSteps {
		if step.Label == "" {
			continue
		}
		durationsByLabel[step.Label] = append(durationsByLabel[step.Label], step.DurationMS)
	}
	return BuildStepDurationStats(durationsByLabel)
}
