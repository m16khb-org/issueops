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

// HistoryService reads, compares and prunes saved self-verification summary
// snapshots through the injected state functions.
type HistoryService struct {
	StateDir func() string
	List     func() (state.ListResult, error)
	Read     func(string) (state.Result, error)
	Delete   func(context.Context, string) error
}

// History lists the saved summary snapshots whose key starts with prefix, newest
// first, returning at most limit entries (0 means no limit). Unreadable or
// unusable records are reported as skipped. When retention.Limit is positive,
// entries beyond that limit are planned for pruning, and deleted only when pruning is both requested and confirmed.
func (service HistoryService) History(ctx context.Context, prefix string, limit int, retention contract.HistoryRetentionOptions) (contract.HistoryResult, error) {
	result := contract.HistoryResult{
		OK:       false,
		StateDir: service.StateDir(),
		Prefix:   prefix,
		Limit:    limit,
		Entries:  []contract.HistoryEntry{},
		Skipped:  []contract.HistorySkipped{},
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
		service.collectRecord(&result, record)
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

// ApplyRetention plans retention over result's entries and records it in
// result.Retention. Candidate keys are deleted only when pruning is both
// requested and confirmed, and each key is read before it is deleted.
func (service HistoryService) ApplyRetention(ctx context.Context, result *contract.HistoryResult, options contract.HistoryRetentionOptions) error {
	keys := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		keys = append(keys, entry.Key)
	}
	decision := domain.PlanHistoryRetention(keys, options.Limit, options.PruneRequested, options.Confirm)
	retention := &contract.HistoryRetention{
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

// Compare reads the baseline and candidate summary snapshots and compares them.
// A regression is reported when the candidate's elapsed time grows by more than
// maxElapsedRegressionPct percent. Invalid keys or unreadable snapshots return an error.
func (service HistoryService) Compare(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (contract.CompareResult, error) {
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
	return domain.CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, service.StateDir()), nil
}

// collectRecord reads one state record and appends it to the history result as
// an entry, or as a skipped record when it cannot be used.
func (service HistoryService) collectRecord(result *contract.HistoryResult, record state.ListEntry) {
	state, err := service.Read(record.Key)
	if err != nil {
		result.Skipped = append(result.Skipped, contract.HistorySkipped{Key: record.Key, Reason: "state_read:" + err.Error()})
		return
	}
	var snapshot contract.StateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		result.Skipped = append(result.Skipped, contract.HistorySkipped{Key: record.Key, Reason: "not_json_summary"})
		return
	}
	skip, warning := domain.HistorySnapshotDiagnostics(record.Key, snapshot)
	if skip != "" {
		result.Skipped = append(result.Skipped, contract.HistorySkipped{Key: record.Key, Reason: skip})
		return
	}
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}
	entry := domain.NewHistoryEntry(snapshot)
	entry.Key, entry.UpdatedAt, entry.Bytes = record.Key, record.UpdatedAt, record.Bytes
	result.Entries = append(result.Entries, entry)
}
