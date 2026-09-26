package summary

import selfverifydomain "issueops/internal/domain/selfverify"

// MapGoalScores projects command steps into the pure scoring model and returns
// the existing summary DTO.
func MapGoalScores(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	definitions := SelfVerificationGoalDefinitions()
	goals := make([]selfverifydomain.GoalDefinition, 0, len(definitions))
	for _, definition := range definitions {
		goals = append(goals, selfverifydomain.GoalDefinition{
			Name: definition.Name, KoreanName: definition.KoreanName, Labels: definition.Labels,
		})
	}
	runs := projectRuns(result)
	domainScores := selfverifydomain.ScoreGoals(goals, runs, result.Iterations, targetScore)
	scores := make([]SelfVerificationGoalScore, 0, len(domainScores))
	for _, score := range domainScores {
		scores = append(scores, SelfVerificationGoalScore{
			Name: score.Name, KoreanName: score.KoreanName, Score: score.Score,
			TargetScore: score.TargetScore, Passed: score.Passed, EvidenceLabels: score.EvidenceLabels,
			PassedChecks: score.PassedChecks, TotalChecks: score.TotalChecks,
		})
	}
	return scores
}

func projectRuns(result SelfAugmentResult) []selfverifydomain.Run {
	runs := make([]selfverifydomain.Run, 0, len(result.Runs))
	for _, run := range result.Runs {
		checks := make([]selfverifydomain.Check, 0, len(run.Steps))
		for _, step := range run.Steps {
			checks = append(checks, selfverifydomain.Check{Label: step.Label, OK: step.OK})
		}
		runs = append(runs, selfverifydomain.Run{Iteration: run.Iteration, Seed: run.Seed, Checks: checks})
	}
	return runs
}
