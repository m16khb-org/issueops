package selfaugment

import (
	"sort"

	contract "issueops/internal/contract/selfaugment"
)

func GoalPassed(score, target float64) bool { return score > target }

func AllGoalsPassed(goals []contract.SelfAugmentGoal) bool {
	if len(goals) == 0 {
		return false
	}
	for _, goal := range goals {
		if !goal.Passed {
			return false
		}
	}
	return true
}

func ScoreBool(ok bool) float64 {
	if ok {
		return 100
	}
	return 0
}

func PrioritizeCandidates(candidates []contract.SelfAugmentCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		leftOpen := candidates[i].Status == contract.CandidateStatusOpen
		rightOpen := candidates[j].Status == contract.CandidateStatusOpen
		if leftOpen != rightOpen {
			return leftOpen
		}
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].ID < candidates[j].ID
	})
}

func SelectedCandidate(candidates []contract.SelfAugmentCandidate) *contract.SelfAugmentCandidate {
	for _, candidate := range candidates {
		if candidate.Status == contract.CandidateStatusOpen {
			copyCandidate := candidate
			return &copyCandidate
		}
	}
	return nil
}

func SelectedCandidateID(candidate *contract.SelfAugmentCandidate) string {
	if candidate == nil {
		return ""
	}
	return candidate.ID
}
