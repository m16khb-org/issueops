package selfaugment

import (
	"slices"
	"sort"

	contract "issueops/internal/contract/selfaugment"
)

// StepDurationStatByLabel indexes stats by label, skipping stats with an empty label; a later
// duplicate overwrites an earlier one.
func StepDurationStatByLabel(stats []contract.StepDurationStat) map[string]contract.StepDurationStat {
	out := map[string]contract.StepDurationStat{}
	for _, stat := range stats {
		if stat.Label == "" {
			continue
		}
		out[stat.Label] = stat
	}
	return out
}

// MaxSlowStepDurationByLabel returns the largest duration in milliseconds seen for each non-empty
// step label.
func MaxSlowStepDurationByLabel(steps []contract.SlowStep) map[string]int64 {
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

// BuildStepDurationStats computes count, min, max, average and nearest-rank p95 duration for each
// non-empty label, sorted by label. A label with no durations yields a stat with only the label
// set; the input slices are not modified.
func BuildStepDurationStats(durationsByLabel map[string][]int64) []contract.StepDurationStat {
	stats := []contract.StepDurationStat{}
	for label, durations := range durationsByLabel {
		if label == "" {
			continue
		}
		if len(durations) == 0 {
			stats = append(stats, contract.StepDurationStat{Label: label})
			continue
		}
		sortedDurations := append([]int64{}, durations...)
		slices.Sort(sortedDurations)
		var sum int64
		for _, duration := range sortedDurations {
			sum += duration
		}
		p95Index := max((95*len(sortedDurations)+99)/100, 1)
		stats = append(stats, contract.StepDurationStat{
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

// StepDurationStatsForCompare returns the summary's recorded step duration stats, or, for older
// summaries without them, stats rebuilt from its slowest steps.
func StepDurationStatsForCompare(summary contract.Summary) []contract.StepDurationStat {
	if len(summary.StepDurationStats) > 0 {
		return append([]contract.StepDurationStat{}, summary.StepDurationStats...)
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
