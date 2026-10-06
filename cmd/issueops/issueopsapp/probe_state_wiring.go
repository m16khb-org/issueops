package issueopsapp

import (
	"context"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification/probe/qagate"
	"issueops/internal/adapter/verification/probe/stateroundtrip"
	"issueops/internal/adapter/verification/probe/stepbudget"
	app "issueops/internal/application/selfaugment"
	statecontract "issueops/internal/contract/state"
	statepath "issueops/internal/domain/statepath"
	"time"
)

func newProbeSnapshotStore() app.SnapshotStore {
	return app.SnapshotStore{NormalizeKey: statepath.NormalizeKey, WriteRecord: probeStateWriteRecord, Now: time.Now}
}
func newStateRoundtripProbe() stateroundtrip.Validator {
	return stateroundtrip.Validator{StateRead: func(dir, key string) (statecontract.StateResult, error) { return newStateService(dir).Read(key) }, WriteRecord: probeStateWriteRecord, WriteSnapshot: newProbeSnapshotStore().Write, OpenDatabase: func(dir string) (stateroundtrip.StateDatabase, error) { return sqlstore.Open(dir) }}
}

// probeStateWriteRecord serves the self-verification probes, whose executor
// API carries no request context; their scratch records are not request-bound.
func probeStateWriteRecord(dir, key string, record statecontract.RecordEnvelope) (string, error) {
	return statestore.WriteStateRecord(context.Background(), dir, key, record)
}

func newStepBudgetProbe() stepbudget.StepBudgetValidationDeps {
	return stepbudget.StepBudgetValidationDeps{WriteSnapshot: newProbeSnapshotStore().Write}
}
func newDocsQAProbe() qagate.Validator {
	return qagate.Validator{ListDocs: newDocsService().List, ListSkills: install.ListSkillNames}
}
