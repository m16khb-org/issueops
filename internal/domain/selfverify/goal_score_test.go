package selfverify

import "testing"

func TestScoreGoalsPreservesMissingIterationsAndExclusiveTarget(t *testing.T) {
	goals := []GoalDefinition{{Name: "test_suite", KoreanName: "테스트 스위트", Labels: []string{"go test", "golden"}}}
	runs := []Run{
		{Iteration: 1, Checks: []Check{{Label: "go test", OK: true}, {Label: "golden", OK: true}}},
		{Iteration: 2, Checks: []Check{{Label: "go test", OK: true}}},
	}
	scores := ScoreGoals(goals, runs, 2, 75)
	if len(scores) != 1 || scores[0].Score != 75 || scores[0].Passed || scores[0].PassedChecks != 3 || scores[0].TotalChecks != 4 {
		t.Fatalf("exclusive threshold=%+v", scores)
	}
	scores = ScoreGoals(goals, runs, 0, 74)
	if len(scores) != 1 || !scores[0].Passed || scores[0].Score != 75 {
		t.Fatalf("fallback run count=%+v", scores)
	}
	scores = ScoreGoals(goals, runs, 3, 50)
	if len(scores) != 1 || scores[0].Score != 50 || scores[0].Passed {
		t.Fatalf("missing third iteration=%+v", scores)
	}
}

func TestScoreGoalsUsesFirstRunAndLastMatchingStep(t *testing.T) {
	goals := []GoalDefinition{{Name: "gate", Labels: []string{"check"}}}
	runs := []Run{
		{Iteration: 1, Checks: []Check{{Label: "check", OK: true}, {Label: "check", OK: false}}},
		{Iteration: 1, Checks: []Check{{Label: "check", OK: true}}},
	}
	scores := ScoreGoals(goals, runs, 1, 0)
	if len(scores) != 1 || scores[0].Score != 0 || scores[0].Passed || scores[0].TotalChecks != 1 {
		t.Fatalf("duplicate run and step precedence=%+v", scores)
	}
}
