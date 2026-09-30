package selfaugment

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/selfaugment"
)

func ValidateHistoryRequest(limit int, retention contract.SelfAugmentHistoryRetentionOptions) error {
	if limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}
	return ValidateHistoryRetention(retention.Limit, retention.PruneRequested, retention.Confirm)
}

func ValidateComparisonRequest(baselineKey, candidateKey string, maxElapsedRegressionPct float64) error {
	if strings.TrimSpace(baselineKey) == "" {
		return fmt.Errorf("baseline-key is required")
	}
	if strings.TrimSpace(candidateKey) == "" {
		return fmt.Errorf("candidate-key is required")
	}
	if maxElapsedRegressionPct < 0 {
		return fmt.Errorf("max elapsed regression pct must be non-negative")
	}
	return nil
}

func HistorySnapshotDiagnostics(key string, snapshot contract.SelfAugmentStateSnapshot) (skipReason, warning string) {
	if !IsSelfVerificationSummaryKind(snapshot.Kind) {
		return "kind:" + snapshot.Kind, ""
	}
	if snapshot.SchemaVersion != 1 {
		return fmt.Sprintf("schema:%d", snapshot.SchemaVersion), ""
	}
	if _, ok := ParseHistoryTimestamp(snapshot.GeneratedAt); !ok {
		return "", "invalid_generated_at:" + key
	}
	return "", ""
}

func NewHistoryEntry(snapshot contract.SelfAugmentStateSnapshot) contract.SelfAugmentHistoryEntry {
	labels := snapshot.Summary.StepLabels
	if labels == nil {
		labels = []string{}
	}
	slowest := snapshot.Summary.SlowestSteps
	if slowest == nil {
		slowest = []contract.SelfAugmentSlowStep{}
	}
	return contract.SelfAugmentHistoryEntry{
		GeneratedAt:  snapshot.GeneratedAt,
		OK:           snapshot.OK,
		Iterations:   snapshot.Iterations,
		BaseSeed:     snapshot.BaseSeed,
		ElapsedMS:    snapshot.ElapsedMS,
		TotalRuns:    snapshot.Summary.TotalRuns,
		TotalSteps:   snapshot.Summary.TotalSteps,
		FailedSteps:  snapshot.Summary.FailedSteps,
		StepLabels:   labels,
		SlowestSteps: slowest,
	}
}
