package issueopsapp

import (
	"issueops/cmd/issueops/selfworkflow/historycompare"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
	stateapp "issueops/internal/application/state"
	"issueops/internal/domain/statepath"
	stateport "issueops/internal/port/state"
)

func newSelfWorkflowHistory(dir string) augmentapp.HistoryService {
	stateDir := func() string { return dir }
	state := stateapp.NewService(stateapp.Dependencies{
		StateDir: stateDir, StatePath: statepath.Path,
		OpenStore:       func(dir string) (stateport.Store, error) { return sqlstore.Open(dir) },
		ExistingRecords: statestore.ExistingRecords{},
	})
	return augmentapp.HistoryService{StateDir: stateDir, List: state.List, Read: state.Read, Delete: state.Delete}
}

func selfWorkflowHistoryCLI() historycompare.CLIDeps {
	service := newSelfWorkflowHistory(statestore.StateDir())
	return historycompare.CLIDeps{History: service.History, Compare: service.Compare, PrintJSON: printJSON}
}
