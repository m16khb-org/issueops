package historycompare

import (
	"time"

	"issueops/internal/domain/selfaugment"
)

func ParseSelfAugmentTimestamp(value string) (time.Time, bool) {
	return selfaugment.ParseHistoryTimestamp(value)
}
