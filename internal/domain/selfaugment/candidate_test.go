package selfaugment

import (
	"testing"

	contract "issueops/internal/contract/selfaugment"
)

func TestCandidateSelectionPrioritizesOpenThenScoreAndID(t *testing.T) {
	candidates := []contract.SelfAugmentCandidate{
		{ID: "satisfied", Status: contract.CandidateStatusSatisfied, Score: 100},
		{ID: "b", Status: contract.CandidateStatusOpen, Score: 80},
		{ID: "a", Status: contract.CandidateStatusOpen, Score: 80},
	}
	PrioritizeCandidates(candidates)
	selected := SelectedCandidate(candidates)
	if selected == nil || selected.ID != "a" || candidates[2].ID != "satisfied" {
		t.Fatalf("priority=%+v selected=%+v", candidates, selected)
	}
	selected.ID = "changed"
	if candidates[0].ID != "a" {
		t.Fatal("selected candidate must be a copy")
	}
}

func TestCandidateSatisfactionAndExclusiveGoal(t *testing.T) {
	candidate := contract.SelfAugmentCandidate{ID: "agent-skill-executor", Status: contract.CandidateStatusOpen, Score: 95}
	MarkSatisfiedCandidate(&candidate, contract.SelfAugmentRepoSignals{HasSelfAugmentSkill: true})
	if candidate.Status != contract.CandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("candidate=%+v", candidate)
	}
	if GoalPassed(95, 95) || !GoalPassed(96, 95) || AllGoalsPassed(nil) {
		t.Fatal("exclusive score gate changed")
	}
}
