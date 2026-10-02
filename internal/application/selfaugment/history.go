package selfaugment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
)

type HistoryService struct {
	StateDir func() string
	List     func() (state.StateListResult, error)
	Read     func(string) (state.StateResult, error)
	Delete   func(context.Context, string) error
}

func (service HistoryService) History(ctx context.Context, prefix string, limit int, retention contract.SelfAugmentHistoryRetentionOptions) (contract.SelfAugmentHistoryResult, error) {
	result := contract.SelfAugmentHistoryResult{
		OK:       false,
		StateDir: service.StateDir(),
		Prefix:   prefix,
		Limit:    limit,
		Entries:  []contract.SelfAugmentHistoryEntry{},
		Skipped:  []contract.SelfAugmentHistorySkipped{},
		Warnings: []string{},
	}
	if err := domain.ValidateHistoryRequest(limit, retention); err != nil {
		return result, err
	}

	list, err := service.List()
	if err != nil {
		return result, err
	}
	for _, record := range list.Records {
		if prefix != "" && !strings.HasPrefix(record.Key, prefix) {
			continue
		}
		state, err := service.Read(record.Key)
		if err != nil {
			result.Skipped = append(result.Skipped, contract.SelfAugmentHistorySkipped{Key: record.Key, Reason: "state_read:" + err.Error()})
			continue
		}
		var snapshot contract.SelfAugmentStateSnapshot
		if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
			result.Skipped = append(result.Skipped, contract.SelfAugmentHistorySkipped{Key: record.Key, Reason: "not_json_summary"})
			continue
		}
		skip, warning := domain.HistorySnapshotDiagnostics(record.Key, snapshot)
		if skip != "" {
			result.Skipped = append(result.Skipped, contract.SelfAugmentHistorySkipped{Key: record.Key, Reason: skip})
			continue
		}
		if warning != "" {
			result.Warnings = append(result.Warnings, warning)
		}
		entry := domain.NewHistoryEntry(snapshot)
		entry.Key, entry.UpdatedAt, entry.Bytes = record.Key, record.UpdatedAt, record.Bytes
		result.Entries = append(result.Entries, entry)
	}
	domain.SortHistoryEntries(result.Entries)
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].Key < result.Skipped[j].Key })
	sort.Strings(result.Warnings)
	result.TotalMatches = len(result.Entries)
	if retention.Limit > 0 {
		if err := service.ApplyRetention(ctx, &result, retention); err != nil {
			return result, err
		}
		sort.Strings(result.Warnings)
	}
	if limit > 0 && len(result.Entries) > limit {
		result.Entries = result.Entries[:limit]
	}
	result.Returned = len(result.Entries)
	result.OK = true
	return result, nil
}

func (service HistoryService) ApplyRetention(ctx context.Context, result *contract.SelfAugmentHistoryResult, options contract.SelfAugmentHistoryRetentionOptions) error {
	keys := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		keys = append(keys, entry.Key)
	}
	decision := domain.PlanHistoryRetention(keys, options.Limit, options.PruneRequested, options.Confirm)
	retention := &contract.SelfAugmentHistoryRetention{
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
	if decision.Apply {
		for _, key := range retention.CandidateKeys {
			if _, err := service.Read(key); err != nil {
				return fmt.Errorf("read retention candidate %q: %w", key, err)
			}
			if err := service.Delete(ctx, key); err != nil {
				return fmt.Errorf("delete retention candidate %q: %w", key, err)
			}
			retention.DeletedKeys = append(retention.DeletedKeys, key)
		}
		retention.Recommendation = domain.DeletedHistoryRecommendation(len(retention.DeletedKeys), options.Limit)
	}
	result.Retention = retention
	return nil
}

func (service HistoryService) Compare(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (contract.SelfAugmentCompareResult, error) {
	result := domain.NewCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct, service.StateDir())
	if err := domain.ValidateComparisonRequest(baselineKey, candidateKey, maxElapsedRegressionPct); err != nil {
		return result, err
	}
	snapshots := SnapshotStore{ReadState: service.Read}
	baseline, err := snapshots.Read(baselineKey)
	if err != nil {
		return result, fmt.Errorf("read baseline summary: %w", err)
	}
	candidate, err := snapshots.Read(candidateKey)
	if err != nil {
		return result, fmt.Errorf("read candidate summary: %w", err)
	}
	return CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, service.StateDir()), nil
}

func CompareSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate contract.SelfAugmentStateSnapshot, stateDir string) contract.SelfAugmentCompareResult {
	NormalizeSnapshotFailureCause(&baseline)
	NormalizeSnapshotFailureCause(&candidate)
	return domain.CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, stateDir)
}
