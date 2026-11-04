package selfaugment

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/selfaugment"
)

// ValidateHistoryRequest rejects a negative limit and any inconsistent retention options (negative
// retention limit, confirm without prune, prune without a positive limit).
func ValidateHistoryRequest(limit int, retention contract.HistoryRetentionOptions) error {
	if limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}
	return validateHistoryRetention(retention.Limit, retention.PruneRequested, retention.Confirm)
}

// ValidateComparisonRequest requires non-blank baseline and candidate keys and a non-negative
// maxElapsedRegressionPct.
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

// HistorySnapshotDiagnostics classifies a stored snapshot for history listing. A non-empty
// skipReason means the snapshot must be skipped (wrong kind or unsupported schema version); a
// non-empty warning means it is listed but its generated_at is unparsable.
func HistorySnapshotDiagnostics(key string, snapshot contract.StateSnapshot) (skipReason, warning string) {
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

// NewHistoryEntry projects snapshot into a history listing entry, replacing missing step labels and
// slowest steps with empty slices.
func NewHistoryEntry(snapshot contract.StateSnapshot) contract.HistoryEntry {
	labels := snapshot.Summary.StepLabels
	if labels == nil {
		labels = []string{}
	}
	slowest := snapshot.Summary.SlowestSteps
	if slowest == nil {
		slowest = []contract.SlowStep{}
	}
	return contract.HistoryEntry{
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
