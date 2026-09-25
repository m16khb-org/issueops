package selfverify

type GoalDefinition struct {
	Name       string
	KoreanName string
	Labels     []string
}

type Check struct {
	Label string
	OK    bool
}

type Run struct {
	Iteration int
	Checks    []Check
}

type GoalScore struct {
	Name           string
	KoreanName     string
	Score          float64
	TargetScore    float64
	Passed         bool
	EvidenceLabels []string
	PassedChecks   int
	TotalChecks    int
}

func ScoreGoals(goals []GoalDefinition, runs []Run, iterations int, targetScore float64) []GoalScore {
	scores := make([]GoalScore, 0, len(goals))
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
		scores = append(scores, GoalScore{
			Name: goal.Name, KoreanName: goal.KoreanName, Score: score, TargetScore: targetScore,
			Passed: score > targetScore, EvidenceLabels: append([]string{}, goal.Labels...),
			PassedChecks: passed, TotalChecks: total,
		})
	}
	return scores
}
