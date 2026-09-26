package historycompare

import (
	"time"

	"issueops/internal/domain/selfaugment"
)

func ParseSelfAugmentTimestamp(value string) (time.Time, bool) {
	return selfaugment.ParseHistoryTimestamp(value)
}

func NonNilStringSlice(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func NonNilSlowStepSlice(items []SelfAugmentSlowStep) []SelfAugmentSlowStep {
	if items == nil {
		return []SelfAugmentSlowStep{}
	}
	return items
}
