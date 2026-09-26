package historycompare

import (
	"fmt"

	"issueops/internal/domain/selfaugment"
)

func ApplySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options SelfAugmentHistoryRetentionOptions) error {
	keys := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		keys = append(keys, entry.Key)
	}
	decision := selfaugment.PlanHistoryRetention(keys, options.Limit, options.PruneRequested, options.Confirm)
	retention := &SelfAugmentHistoryRetention{
		Enabled:        true,
		Limit:          options.Limit,
		TotalMatches:   result.TotalMatches,
		RetainedKeys:   decision.RetainedKeys,
		CandidateKeys:  decision.CandidateKeys,
		DeletedKeys:    []string{},
		PruneRequested: decision.PruneRequested,
		Confirm:        decision.Confirm,
		DryRun:         decision.DryRun,
		Recommendation: decision.Recommendation,
	}
	if decision.Warning != "" {
		result.Warnings = append(result.Warnings, decision.Warning)
	}
	if options.PruneRequested && options.Confirm {
		for _, key := range retention.CandidateKeys {
			if _, err := StateRead(key); err != nil {
				return fmt.Errorf("read retention candidate %q: %w", key, err)
			}
			if err := StateDelete(key); err != nil {
				return fmt.Errorf("delete retention candidate %q: %w", key, err)
			}
			retention.DeletedKeys = append(retention.DeletedKeys, key)
		}
		retention.Recommendation = selfaugment.DeletedHistoryRecommendation(len(retention.DeletedKeys), options.Limit)
	}
	result.Retention = retention
	return nil
}
