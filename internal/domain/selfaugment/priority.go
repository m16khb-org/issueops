package selfaugment

import (
	"sort"

	contract "issueops/internal/contract/selfaugment"
)

func goalPassed(score, target float64) bool { return score > target }

// AllGoalsPassed reports whether goals is non-empty and every goal has Passed set.
func AllGoalsPassed(goals []contract.Goal) bool {
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

// ScoreBool maps ok to a goal score: 100 when true, 0 otherwise.
func ScoreBool(ok bool) float64 {
	if ok {
		return 100
	}
	return 0
}

func prioritizeCandidates(candidates []contract.Candidate) {
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

func selectedCandidate(candidates []contract.Candidate) *contract.Candidate {
	for _, candidate := range candidates {
		if candidate.Status == contract.CandidateStatusOpen {
			copyCandidate := candidate
			return &copyCandidate
		}
	}
	return nil
}

// SelectedCandidateID returns the ID of candidate, or an empty string when candidate is nil.
func SelectedCandidateID(candidate *contract.Candidate) string {
	if candidate == nil {
		return ""
	}
	return candidate.ID
}
