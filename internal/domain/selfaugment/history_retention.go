package selfaugment

import "fmt"

// HistoryRetentionDecision is the outcome of planning history retention: which checkpoint keys fall
// within the budget, which are prune candidates, whether the prune is applied or only a dry run,
// and the recommendation and warning text to report.
type HistoryRetentionDecision struct {
	Apply          bool
	RetainedKeys   []string
	CandidateKeys  []string
	PruneRequested bool
	Confirm        bool
	DryRun         bool
	Recommendation string
	Warning        string
}

// PlanHistoryRetention splits keys, assumed newest first, into the first limit retained keys and
// the remaining prune candidates. Pruning is applied only when both pruneRequested and confirm are
// set; a request without confirm is a dry run.
func PlanHistoryRetention(keys []string, limit int, pruneRequested, confirm bool) HistoryRetentionDecision {
	decision := HistoryRetentionDecision{
		Apply:          pruneRequested && confirm,
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

func validateHistoryRetention(limit int, pruneRequested, confirm bool) error {
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

// DeletedHistoryRecommendation returns the recommendation text reported after deleted checkpoints
// beyond limit were removed.
func DeletedHistoryRecommendation(deleted, limit int) string {
	return fmt.Sprintf("deleted %d history checkpoint(s) beyond retention-limit=%d", deleted, limit)
}
