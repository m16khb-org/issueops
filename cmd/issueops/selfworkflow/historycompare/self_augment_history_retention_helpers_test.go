package historycompare

import "context"

func ApplySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options SelfAugmentHistoryRetentionOptions) error {
	return historyService().ApplyRetention(context.Background(), result, options)
}
