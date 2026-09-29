package selfaugment

import (
	contract "issueops/internal/contract/selfaugment"
	"testing"
)

func TestPlanRequiresEveryGoalAboveTargetAndSelectsAfterPenalty(t *testing.T) {
	for _, target := range []float64{95, 100} {
		facts := PlanFacts{Signals: contract.SelfAugmentRepoSignals{HasGeniusThink: true}, DocsOK: true, ImplementationDelta: true, VerificationPassed: true, LessonsCaptured: true}
		candidates := []contract.SelfAugmentCandidate{{ID: "repeat", Status: contract.CandidateStatusOpen, Score: 90}, {ID: "next", Status: contract.CandidateStatusOpen, Score: 80}}
		result := NewPlan(contract.SelfAugmentPlanRequest{TargetScore: target}, facts, candidates, map[string]int{"repeat": 2})
		if result.TerminationEligible != (target < 100) {
			t.Fatalf("target=%v termination=%v", target, result.TerminationEligible)
		}
		if result.SelectedCandidate == nil || result.SelectedCandidate.ID != "next" || len(result.Warnings) != 1 {
			t.Fatalf("penalty must precede selection: %+v", result)
		}
	}
	for i := range 4 {
		facts := PlanFacts{Signals: contract.SelfAugmentRepoSignals{HasGeniusThink: true}, DocsOK: true, ImplementationDelta: true, VerificationPassed: true, LessonsCaptured: true}
		switch i {
		case 0:
			facts.DocsOK = false
		case 1:
			facts.ImplementationDelta = false
		case 2:
			facts.VerificationPassed = false
		case 3:
			facts.LessonsCaptured = false
		}
		result := NewPlan(contract.SelfAugmentPlanRequest{TargetScore: 95}, facts, nil, nil)
		if result.TerminationEligible || result.Goals[i].Score != 0 || result.Goals[i].Passed {
			t.Fatalf("failed observable admitted: %+v", result.Goals)
		}
	}
}
