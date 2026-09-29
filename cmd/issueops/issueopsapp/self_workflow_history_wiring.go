package issueopsapp

import (
	"issueops/cmd/issueops/selfworkflow/historycompare"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
)

func newSelfWorkflowHistory(dir string) augmentapp.HistoryService {
	stateDir := func() string { return dir }
	state := newStateService(dir)
	return augmentapp.HistoryService{StateDir: stateDir, List: state.List, Read: state.Read, Delete: state.Delete}
}

func selfWorkflowHistoryCLI() historycompare.CLIDeps {
	service := newSelfWorkflowHistory(statestore.StateDir())
	return historycompare.CLIDeps{History: service.History, Compare: service.Compare, PrintJSON: printJSON}
}
