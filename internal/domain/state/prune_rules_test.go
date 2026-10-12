package state

import (
	"testing"
	"time"

	statecontract "issueops/internal/contract/state"
)

func TestSelectPrunePreservesPrefixAgeAndCountRules(t *testing.T) {
	records := []statecontract.StateListEntry{
		{Key: "other-old", UpdatedAt: "2026-01-01T00:00:00Z"},
		{Key: "trace-old", UpdatedAt: "2026-01-01T00:00:00Z"},
		{Key: "trace-newer", UpdatedAt: "2026-01-03T00:00:00Z"},
		{Key: "trace-newest", UpdatedAt: "2026-01-04T00:00:00Z"},
	}
	pruned, kept := SelectPrune(records, "trace-", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), 1)
	if got, want := entryKeys(pruned), []string{"trace-newer", "trace-old"}; !sameStrings(got, want) {
		t.Fatalf("pruned=%v want %v", got, want)
	}
	if got, want := entryKeys(kept), []string{"other-old", "trace-newest"}; !sameStrings(got, want) {
		t.Fatalf("kept=%v want %v", got, want)
	}
}

func entryKeys(entries []statecontract.StateListEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func sameStrings(left, right []string) bool {
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

func TestSelectPruneCountRuleDropsOldestByTimestamp(t *testing.T) {
	// Keys sort opposite to timestamps, so the retained-count rule must order
	// by parsed time.
	records := []statecontract.StateListEntry{
		{Key: "a", UpdatedAt: "2026-01-05T00:00:00Z"},
		{Key: "b", UpdatedAt: "2026-01-04T00:00:00Z"},
		{Key: "c", UpdatedAt: "2026-01-03T00:00:00Z"},
	}
	pruned, _ := SelectPrune(records, "", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 2)
	if !sameStrings(entryKeys(pruned), []string{"c"}) {
		t.Fatalf("pruned=%v", entryKeys(pruned))
	}
}

func TestSelectPruneAgeRuleAppliesWithoutRetainedCountLimit(t *testing.T) {
	records := []statecontract.StateListEntry{
		{Key: "old", UpdatedAt: "2026-01-01T00:00:00Z"},
		{Key: "boundary", UpdatedAt: "2026-01-02T00:00:00Z"},
		{Key: "new", UpdatedAt: "2026-01-03T00:00:00Z"},
		{Key: "bad", UpdatedAt: "invalid"},
	}
	pruned, kept := SelectPrune(records, "", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), 0)
	if !sameStrings(entryKeys(pruned), []string{"old"}) || !sameStrings(entryKeys(kept), []string{"bad", "boundary", "new"}) {
		t.Fatalf("pruned=%v kept=%v", entryKeys(pruned), entryKeys(kept))
	}
}
