package selfverify

import (
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

func MapGoalScores(result augment.SelfAugmentResult, targetScore float64) []verify.SelfVerificationGoalScore {
	return domain.ScoreGoals(domain.GoalDefinitions(), projectRuns(result), result.Iterations, targetScore)
}

func projectRuns(result augment.SelfAugmentResult) []domain.Run {
	runs := make([]domain.Run, 0, len(result.Runs))
	for _, run := range result.Runs {
		checks := make([]domain.Check, 0, len(run.Steps))
		for _, step := range run.Steps {
			checks = append(checks, domain.Check{Label: step.Label, OK: step.OK})
		}
		runs = append(runs, domain.Run{Iteration: run.Iteration, Seed: run.Seed, Checks: checks})
	}
	return runs
}
