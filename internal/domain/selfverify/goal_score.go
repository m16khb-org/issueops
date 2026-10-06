package selfverify

import contract "issueops/internal/contract/selfverify"

type Check struct {
	Label string
	OK    bool
}

type Run struct {
	Iteration int
	Seed      int64
	Checks    []Check
}

func ScoreGoals(goals []contract.SelfVerificationGoalDefinition, runs []Run, iterations int, targetScore float64) []contract.SelfVerificationGoalScore {
	scores := make([]contract.SelfVerificationGoalScore, 0, len(goals))
	runCount := iterations
	if runCount < 1 {
		runCount = len(runs)
	}
	for _, goal := range goals {
		passed := 0
		total := 0
		for iteration := 1; iteration <= runCount; iteration++ {
			steps := map[string]bool{}
			for _, run := range runs {
				if run.Iteration != iteration {
					continue
				}
				for _, check := range run.Checks {
					steps[check.Label] = check.OK
				}
				break
			}
			for _, label := range goal.Labels {
				total++
				if steps[label] {
					passed++
				}
			}
		}
		score := 0.0
		if total > 0 {
			score = float64(passed) * 100 / float64(total)
		}
		scores = append(scores, contract.SelfVerificationGoalScore{
			Name: goal.Name, KoreanName: goal.KoreanName, Score: score, TargetScore: targetScore,
			Passed: score > targetScore, EvidenceLabels: append([]string{}, goal.Labels...),
			PassedChecks: passed, TotalChecks: total,
		})
	}
	return scores
}
