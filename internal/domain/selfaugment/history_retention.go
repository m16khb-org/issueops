package selfaugment

import "fmt"

type HistoryRetentionDecision struct {
	RetainedKeys   []string
	CandidateKeys  []string
	PruneRequested bool
	Confirm        bool
	DryRun         bool
	Recommendation string
	Warning        string
}

func PlanHistoryRetention(keys []string, limit int, pruneRequested, confirm bool) HistoryRetentionDecision {
	decision := HistoryRetentionDecision{
		RetainedKeys:   []string{},
		CandidateKeys:  []string{},
		PruneRequested: pruneRequested,
		Confirm:        confirm,
		DryRun:         pruneRequested && !confirm,
		Recommendation: "within_retention_budget",
	}
	for i, key := range keys {
		if i < limit {
			decision.RetainedKeys = append(decision.RetainedKeys, key)
		} else {
			decision.CandidateKeys = append(decision.CandidateKeys, key)
		}
	}
	if len(decision.CandidateKeys) > 0 {
		decision.Recommendation = fmt.Sprintf("prune %d history checkpoint(s) beyond retention-limit=%d after reviewing dry-run output", len(decision.CandidateKeys), limit)
		decision.Warning = fmt.Sprintf("history_retention_candidates:%d", len(decision.CandidateKeys))
	}
	return decision
}

func ValidateHistoryRetention(limit int, pruneRequested, confirm bool) error {
	if limit < 0 {
		return fmt.Errorf("retention-limit must be non-negative")
	}
	if confirm && !pruneRequested {
		return fmt.Errorf("confirm requires --prune-retention")
	}
	if pruneRequested && limit <= 0 {
		return fmt.Errorf("prune-retention requires a positive --retention-limit")
	}
	return nil
}

func DeletedHistoryRecommendation(deleted, limit int) string {
	return fmt.Sprintf("deleted %d history checkpoint(s) beyond retention-limit=%d", deleted, limit)
}
