package augmentcatalog

import (
	domain "issueops/internal/domain/selfaugment"
)

func ScoreBool(ok bool) float64                              { return domain.ScoreBool(ok) }
func AllSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool { return domain.AllGoalsPassed(goals) }
func SelectedCandidateID(candidate *SelfAugmentCandidate) string {
	return domain.SelectedCandidateID(candidate)
}

func SelectGeniusFormulas(text string) []string { return domain.SelectGeniusFormulas(text) }

func SelfAugmentResearchInfluences() []SelfAugmentInfluence { return domain.ResearchInfluences() }
