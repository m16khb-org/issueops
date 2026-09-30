package issueopsapp

import (
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification/probe/qagate"
	"issueops/internal/adapter/verification/probe/stateroundtrip"
	"issueops/internal/adapter/verification/probe/stepbudget"
	app "issueops/internal/application/selfaugment"
	statecontract "issueops/internal/contract/state"
	"time"
)

func newProbeSnapshotStore() app.SnapshotStore {
	return app.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}
}
func newStateRoundtripProbe() stateroundtrip.Validator {
	return stateroundtrip.Validator{StateRead: func(dir, key string) (statecontract.StateResult, error) { return newStateService(dir).Read(key) }, WriteRecord: statestore.WriteStateRecord, WriteSnapshot: newProbeSnapshotStore().Write, OpenDatabase: func(dir string) (stateroundtrip.StateDatabase, error) { return sqlstore.Open(dir) }}
}
func newStepBudgetProbe() stepbudget.StepBudgetValidationDeps {
	return stepbudget.StepBudgetValidationDeps{WriteSnapshot: newProbeSnapshotStore().Write}
}
func newDocsQAProbe() qagate.Validator {
	return qagate.Validator{ListDocs: newDocsService().List, ListSkills: install.ListSkillNames}
}
