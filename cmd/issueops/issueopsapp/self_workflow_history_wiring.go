package issueopsapp

import (
	"context"
	"issueops/cmd/issueops/selfworkflow/historycompare"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
)

func newSelfWorkflowHistory(dir string) augmentapp.HistoryService {
	stateDir := func() string { return dir }
	state := newStateService(dir)
	return augmentapp.HistoryService{StateDir: stateDir, List: state.List, Read: state.Read, Delete: state.Delete}
}

func selfWorkflowHistoryCLI() historycompare.CLIDeps {
	service := newSelfWorkflowHistory(statestore.StateDir())
	return historycompare.CLIDeps{History: func(prefix string, limit int, retention contract.SelfAugmentHistoryRetentionOptions) (contract.SelfAugmentHistoryResult, error) {
		return service.History(context.Background(), prefix, limit, retention)
	}, Compare: service.Compare, PrintJSON: printJSON}
}
