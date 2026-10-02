package historycompare

import (
	"context"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
)

func historyService() app.HistoryService {
	return app.HistoryService{StateDir: statestore.StateDir, List: statestore.StateList, Read: statestore.StateRead, Delete: statestore.StateDelete}
}

func SelfAugmentHistory(prefix string, limit int, retentionOptions ...SelfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
	retention := SelfAugmentHistoryRetentionOptions{}
	if len(retentionOptions) > 0 {
		retention = retentionOptions[0]
	}
	return historyService().History(context.Background(), prefix, limit, retention)
}
