package selfverify

import (
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

func MapGoalScores(result augment.SelfAugmentResult, targetScore float64) []verify.SelfVerificationGoalScore {
	definitions := domain.GoalDefinitions()
	goals := make([]domain.GoalDefinition, 0, len(definitions))
	for _, definition := range definitions {
		goals = append(goals, domain.GoalDefinition{
			Name: definition.Name, KoreanName: definition.KoreanName, Labels: definition.Labels,
		})
	}
	runs := projectRuns(result)
	domainScores := domain.ScoreGoals(goals, runs, result.Iterations, targetScore)
	scores := make([]verify.SelfVerificationGoalScore, 0, len(domainScores))
	for _, score := range domainScores {
		scores = append(scores, verify.SelfVerificationGoalScore{
			Name: score.Name, KoreanName: score.KoreanName, Score: score.Score,
			TargetScore: score.TargetScore, Passed: score.Passed, EvidenceLabels: score.EvidenceLabels,
			PassedChecks: score.PassedChecks, TotalChecks: score.TotalChecks,
		})
	}
	return scores
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
