package selfverify

import contract "issueops/internal/contract/selfverify"

// Check is the outcome of one labeled step within a Run.
type Check struct {
	Label string
	OK    bool
}

// Run records the checks of one self-verify iteration, identified by its 1-based Iteration and its Seed.
type Run struct {
	Iteration int
	Seed      int64
	Checks    []Check
}

// ScoreGoals scores each goal as the percentage of its labeled checks that passed across iterations
// (or len(runs) when iterations is below 1). A goal passes when its score exceeds targetScore, and is
// marked NotRun when none of its labels executed.
func ScoreGoals(goals []contract.SelfVerificationGoalDefinition, runs []Run, iterations int, targetScore float64) []contract.SelfVerificationGoalScore {
	scores := make([]contract.SelfVerificationGoalScore, 0, len(goals))
	runCount := iterations
	if runCount < 1 {
		runCount = len(runs)
	}
	for _, goal := range goals {
		passed := 0
		total := 0
		executed := 0
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
				ok, ran := steps[label]
				if ran {
					executed++
				}
				if ok {
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
			PassedChecks: passed, TotalChecks: total, NotRun: total > 0 && executed == 0,
		})
	}
	return scores
}
