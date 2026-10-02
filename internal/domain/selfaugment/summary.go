package selfaugment

import (
	"sort"

	contract "issueops/internal/contract/selfaugment"
)

type SummaryStep struct {
	Label      string
	OK         bool
	Reused     bool
	DurationMS int64
}

type SummaryRun struct {
	Iteration int
	Seed      int64
	Steps     []SummaryStep
}

func SummarizeSteps(runs []SummaryRun, targetScore float64) contract.SelfAugmentSummary {
	summary := contract.SelfAugmentSummary{
		TotalRuns:         len(runs),
		TargetScore:       targetScore,
		StepLabels:        []string{},
		SlowestSteps:      []contract.SelfAugmentSlowStep{},
		StepDurationStats: []contract.SelfAugmentStepDurationStat{},
	}
	seenLabels := map[string]bool{}
	durationsByLabel := map[string][]int64{}
	reusedByLabel := map[string]int{}
	for _, run := range runs {
		for _, step := range run.Steps {
			summary.TotalSteps++
			if step.OK {
				summary.PassedSteps++
			} else {
				summary.FailedSteps++
				if summary.FailedStep == "" {
					summary.FailedIteration = run.Iteration
					summary.FailedSeed = run.Seed
					summary.FailedStep = step.Label
				}
			}
			if !seenLabels[step.Label] {
				seenLabels[step.Label] = true
				summary.StepLabels = append(summary.StepLabels, step.Label)
				durationsByLabel[step.Label] = nil
			}
			if step.Reused {
				reusedByLabel[step.Label]++
				continue
			}
			summary.SlowestSteps = append(summary.SlowestSteps, contract.SelfAugmentSlowStep{
				Iteration: run.Iteration, Seed: run.Seed, Label: step.Label, DurationMS: step.DurationMS,
			})
			durationsByLabel[step.Label] = append(durationsByLabel[step.Label], step.DurationMS)
		}
	}
	sort.Slice(summary.SlowestSteps, func(i, j int) bool {
		if summary.SlowestSteps[i].DurationMS != summary.SlowestSteps[j].DurationMS {
			return summary.SlowestSteps[i].DurationMS > summary.SlowestSteps[j].DurationMS
		}
		if summary.SlowestSteps[i].Iteration != summary.SlowestSteps[j].Iteration {
			return summary.SlowestSteps[i].Iteration < summary.SlowestSteps[j].Iteration
		}
		return summary.SlowestSteps[i].Label < summary.SlowestSteps[j].Label
	})
	if len(summary.SlowestSteps) > 5 {
		summary.SlowestSteps = summary.SlowestSteps[:5]
	}
	summary.StepDurationStats = BuildStepDurationStats(durationsByLabel)
	for i := range summary.StepDurationStats {
		summary.StepDurationStats[i].ReusedCount = reusedByLabel[summary.StepDurationStats[i].Label]
	}
	return summary
}

func FinalizeSummary(summary *contract.SelfAugmentSummary, runOK bool) {
	summary.MinimumGoalScore = 100
	if len(summary.GoalScores) == 0 {
		summary.MinimumGoalScore = 0
	}
	summary.TerminationEligible = runOK
	for _, goal := range summary.GoalScores {
		if goal.Score < summary.MinimumGoalScore {
			summary.MinimumGoalScore = goal.Score
		}
		if !goal.Passed {
			summary.TerminationEligible = false
		}
	}
}
