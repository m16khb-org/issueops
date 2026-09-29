package historycompare

func ApplySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options SelfAugmentHistoryRetentionOptions) error {
	return historyService().ApplyRetention(result, options)
}
