package stateroundtrip

import (
	selfverify "issueops/internal/contract/selfverify"

	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	stateapp "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/statepath"
	stateport "issueops/internal/port/state"
	"time"
)

func testValidator() Validator {
	store := app.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}
	return Validator{StateRead: readProbeState, WriteRecord: statestore.WriteStateRecord, WriteSnapshot: store.Write, OpenDatabase: func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }}
}
func testStateDependencies(deps stateRoundtripValidationDeps) stateRoundtripValidationDeps {
	v := testValidator()
	if deps.stateRead == nil {
		deps.stateRead = v.StateRead
	}
	if deps.writeRecord == nil {
		deps.writeRecord = v.WriteRecord
	}
	if deps.writeSnapshot == nil {
		deps.writeSnapshot = v.WriteSnapshot
	}
	if deps.openDatabase == nil {
		deps.openDatabase = v.OpenDatabase
	}
	return deps
}
func validateStateRoundtripWithTestDeps(binary, root string, seed int64, deps stateRoundtripValidationDeps) selfverify.StepResult {
	return validateStateRoundtripWithDeps(binary, root, seed, testStateDependencies(deps))
}

func readProbeState(dir, key string) (statecontract.StateResult, error) {
	service := stateapp.NewService(stateapp.Dependencies{StateDir: func() string { return dir }, StatePath: statepath.Path, OpenStore: func(dir string) (stateport.Store, error) { return sqlstore.Open(dir) }, ExistingRecords: statestore.ExistingRecords{}})
	return service.Read(key)
}
