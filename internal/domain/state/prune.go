package state

import (
	"sort"
	"strings"
	"time"

	statecontract "issueops/internal/contract/state"
)

// SelectPrune separates records selected by prefix, age, and retained-count
// policy from records that must remain. Both outputs are sorted by key.
func SelectPrune(records []statecontract.StateListEntry, prefix string, cutoff time.Time, maxRecords int) ([]statecontract.StateListEntry, []statecontract.StateListEntry) {
	matching := make([]statecontract.StateListEntry, 0, len(records))
	kept := make([]statecontract.StateListEntry, 0, len(records))
	for _, record := range records {
		if prefix == "" || strings.HasPrefix(record.Key, prefix) {
			matching = append(matching, record)
			continue
		}
		kept = append(kept, record)
	}
	sort.Slice(matching, func(i, j int) bool {
		left, leftErr := statecontract.ParseTime(matching[i].UpdatedAt)
		right, rightErr := statecontract.ParseTime(matching[j].UpdatedAt)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.Before(right)
		}
		return matching[i].Key < matching[j].Key
	})
	selected := selectPruneKeys(matching, cutoff, maxRecords)
	pruned := make([]statecontract.StateListEntry, 0, len(selected))
	for _, record := range matching {
		if selected[record.Key] {
			pruned = append(pruned, record)
		} else {
			kept = append(kept, record)
		}
	}
	sort.Slice(pruned, func(i, j int) bool { return pruned[i].Key < pruned[j].Key })
	sort.Slice(kept, func(i, j int) bool { return kept[i].Key < kept[j].Key })
	return pruned, kept
}

// selectPruneKeys는 cutoff보다 오래된 키와, 보존 개수를 넘는 가장 오래된 키를 고른다.
// matching은 오래된 순으로 정렬되어 있어야 한다.
func selectPruneKeys(matching []statecontract.StateListEntry, cutoff time.Time, maxRecords int) map[string]bool {
	selected := make(map[string]bool, len(matching))
	for _, record := range matching {
		updatedAt, err := statecontract.ParseTime(record.UpdatedAt)
		if err == nil && !updatedAt.IsZero() && updatedAt.Before(cutoff) {
			selected[record.Key] = true
		}
	}
	if maxRecords > 0 {
		retained := make([]statecontract.StateListEntry, 0, len(matching))
		for _, record := range matching {
			if !selected[record.Key] {
				retained = append(retained, record)
			}
		}
		for len(retained) > maxRecords {
			selected[retained[0].Key] = true
			retained = retained[1:]
		}
	}
	return selected
}
