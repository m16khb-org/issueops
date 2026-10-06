package historycompare

import (
	augmentcontract "issueops/internal/contract/selfaugment"
	"testing"

	augmentdomain "issueops/internal/domain/selfaugment"
)

func TestSelfAugmentHistoryCoversInvalidTimestampSchemaSkipAndNilSlices(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	if err := writeSnapshotForTest(dir, "self-verify-invalid-time", augmentcontract.SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          augmentdomain.SelfVerificationSummaryKind,
		OK:            true,
		GeneratedAt:   "not-a-time",
		Summary:       augmentcontract.SelfAugmentSummary{TotalRuns: 1, TotalSteps: 1},
	}); err != nil {
		t.Fatalf("write invalid time: %v", err)
	}
	if err := writeSnapshotForTest(dir, "self-verify-bad-schema", augmentcontract.SelfAugmentStateSnapshot{
		SchemaVersion: 2,
		Kind:          augmentdomain.SelfVerificationSummaryKind,
		Summary:       augmentcontract.SelfAugmentSummary{TotalRuns: 1},
	}); err != nil {
		t.Fatalf("write bad schema: %v", err)
	}

	result, err := SelfAugmentHistory("self-verify", 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if !containsString(result.Warnings, "invalid_generated_at:self-verify-invalid-time") {
		t.Fatalf("missing invalid timestamp warning: %+v", result.Warnings)
	}
	if !historySkippedKey(result.Skipped, "self-verify-bad-schema") {
		t.Fatalf("missing bad schema skip: %+v", result.Skipped)
	}
	if len(result.Entries) != 1 || result.Entries[0].StepLabels == nil || result.Entries[0].SlowestSteps == nil {
		t.Fatalf("expected one entry with non-nil slices: %+v", result.Entries)
	}
}

func TestParseSelfAugmentTimestampCoversEmptyInvalidAndRFC3339Fallback(t *testing.T) {
	if _, ok := augmentdomain.ParseHistoryTimestamp(""); ok {
		t.Fatal("empty timestamp parsed")
	}
	if _, ok := augmentdomain.ParseHistoryTimestamp("not-a-time"); ok {
		t.Fatal("invalid timestamp parsed")
	}
	parsed, ok := augmentdomain.ParseHistoryTimestamp("2000-01-01T00:00:00Z")
	if !ok || parsed.Year() != 2000 {
		t.Fatalf("expected RFC3339 timestamp to parse, parsed=%v ok=%v", parsed, ok)
	}
}
