package historycompare

import app "issueops/internal/application/selfaugment"

func historyService() app.HistoryService {
	return app.HistoryService{StateDir: StateDir, List: StateList, Read: StateRead, Delete: StateDelete}
}

func SelfAugmentHistory(prefix string, limit int, retentionOptions ...SelfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
	retention := SelfAugmentHistoryRetentionOptions{}
	if len(retentionOptions) > 0 {
		retention = retentionOptions[0]
	}
	return historyService().History(prefix, limit, retention)
}
