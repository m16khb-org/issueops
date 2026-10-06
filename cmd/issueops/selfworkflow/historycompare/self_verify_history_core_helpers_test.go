package historycompare

import (
	"context"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func historyService() app.HistoryService {
	return app.HistoryService{StateDir: statestore.StateDir, List: statestore.NewService().List, Read: statestore.NewService().Read, Delete: statestore.NewService().Delete}
}

func SelfAugmentHistory(prefix string, limit int, retentionOptions ...augmentcontract.SelfAugmentHistoryRetentionOptions) (augmentcontract.SelfAugmentHistoryResult, error) {
	retention := augmentcontract.SelfAugmentHistoryRetentionOptions{}
	if len(retentionOptions) > 0 {
		retention = retentionOptions[0]
	}
	return historyService().History(context.Background(), prefix, limit, retention)
}
