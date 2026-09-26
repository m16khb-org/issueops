package selfaugment

import (
	"sort"
	"strings"
	"time"

	contract "issueops/internal/contract/selfaugment"
)

func ParseHistoryTimestamp(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func SortHistoryEntries(entries []contract.SelfAugmentHistoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left, leftOK := ParseHistoryTimestamp(entries[i].GeneratedAt)
		right, rightOK := ParseHistoryTimestamp(entries[j].GeneratedAt)
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && !left.Equal(right) {
			return left.After(right)
		}
		leftUpdated, leftUpdatedOK := ParseHistoryTimestamp(entries[i].UpdatedAt)
		rightUpdated, rightUpdatedOK := ParseHistoryTimestamp(entries[j].UpdatedAt)
		if leftUpdatedOK != rightUpdatedOK {
			return leftUpdatedOK
		}
		if leftUpdatedOK && !leftUpdated.Equal(rightUpdated) {
			return leftUpdated.After(rightUpdated)
		}
		return entries[i].Key < entries[j].Key
	})
}
