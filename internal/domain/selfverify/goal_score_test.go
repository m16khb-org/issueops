package selfverify

import (
	contract "issueops/internal/contract/selfverify"
	"testing"
)

func TestScoreGoalsPreservesMissingIterationsAndExclusiveTarget(t *testing.T) {
	goals := []contract.SelfVerificationGoalDefinition{{Name: "test_suite", KoreanName: "테스트 스위트", Labels: []string{"go test", "golden"}}}
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
	goals := []contract.SelfVerificationGoalDefinition{{Name: "gate", Labels: []string{"check"}}}
	runs := []Run{
		{Iteration: 1, Checks: []Check{{Label: "check", OK: true}, {Label: "check", OK: false}}},
		{Iteration: 1, Checks: []Check{{Label: "check", OK: true}}},
	}
	scores := ScoreGoals(goals, runs, 1, 0)
	if len(scores) != 1 || scores[0].Score != 0 || scores[0].Passed || scores[0].TotalChecks != 1 {
		t.Fatalf("duplicate run and step precedence=%+v", scores)
	}
}

func TestScoreGoalsGoalWithoutLabelsScoresZero(t *testing.T) {
	goals := []contract.SelfVerificationGoalDefinition{{Name: "empty"}}
	scores := ScoreGoals(goals, []Run{{Iteration: 1}}, 1, 90)
	if len(scores) != 1 || scores[0].Score != 0 || scores[0].Passed || scores[0].TotalChecks != 0 {
		t.Fatalf("scores=%+v", scores)
	}
}

func TestScoreGoalsMarksGoalsWhoseChecksNeverRan(t *testing.T) {
	goals := []contract.SelfVerificationGoalDefinition{
		{Name: "failed", Labels: []string{"a"}},
		{Name: "partial", Labels: []string{"a", "b"}},
		{Name: "never", Labels: []string{"b", "c"}},
		{Name: "empty"},
	}
	runs := []Run{{Iteration: 1, Checks: []Check{{Label: "a", OK: false}}}}
	got := map[string]bool{}
	for _, score := range ScoreGoals(goals, runs, 1, 95) {
		got[score.Name] = score.NotRun
		if score.Passed {
			t.Fatalf("%s passed without passing checks: %+v", score.Name, score)
		}
	}
	want := map[string]bool{"failed": false, "partial": false, "never": true, "empty": false}
	for name, notRun := range want {
		if got[name] != notRun {
			t.Fatalf("%s not_run=%v, want %v (all=%v)", name, got[name], notRun, got)
		}
	}
}
