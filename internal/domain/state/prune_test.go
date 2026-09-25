package state

import (
	"testing"
	"time"

	statecontract "issueops/internal/contract/state"
)

func TestSelectPrunePreservesPrefixAgeCountAndInvalidTimestamp(t *testing.T) {
	records := []statecontract.StateListEntry{
		{Key: "other-old", UpdatedAt: "2026-01-01T00:00:00Z"},
		{Key: "trace-old", UpdatedAt: "2026-01-01T00:00:00Z"},
		{Key: "trace-newer", UpdatedAt: "2026-01-03T00:00:00Z"},
		{Key: "trace-newest", UpdatedAt: "2026-01-04T00:00:00Z"},
		{Key: "trace-invalid", UpdatedAt: "invalid"},
	}
	pruned, kept := SelectPrune(records, "trace-", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), 2)
	// An unparseable timestamp is not age-pruned, but still participates in the
	// existing retained-count ordering by key.
	if got, want := pruneKeys(pruned), []string{"trace-invalid", "trace-old"}; !pruneKeysEqual(got, want) {
		t.Fatalf("pruned=%v want=%v", got, want)
	}
	if got, want := pruneKeys(kept), []string{"other-old", "trace-newer", "trace-newest"}; !pruneKeysEqual(got, want) {
		t.Fatalf("kept=%v want=%v", got, want)
	}
}

func pruneKeys(entries []statecontract.StateListEntry) []string {
	keys := make([]string, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	return keys
}

func pruneKeysEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
