package probe

import (
	selfaugment "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"

	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification/probe/qagate"
	"issueops/internal/adapter/verification/probe/stateroundtrip"
	"issueops/internal/adapter/verification/probe/stepbudget"
	docsapp "issueops/internal/application/docs"
	app "issueops/internal/application/selfaugment"
	stateapp "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/statepath"
	stateport "issueops/internal/port/state"
	"time"
)

func testSnapshotStore() app.SnapshotStore {
	return app.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}
}
func ValidateStateRoundtrip(binary, root string, seed int64) selfverify.StepResult {
	return (stateroundtrip.Validator{StateRead: readProbeState, WriteRecord: statestore.WriteStateRecord, WriteSnapshot: testSnapshotStore().Write, OpenDatabase: func(dir string) (stateroundtrip.StateDatabase, error) { return sqlstore.Open(dir) }}).Validate(binary, root, seed)
}

func ValidateStepBudgetBaseline(binary, root string, seed int64) selfverify.StepResult {
	return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, stepbudget.StepBudgetValidationDeps{WriteSnapshot: testSnapshotStore().Write})
}
func writeSelfAugmentSnapshotRecord(dir, key string, snapshot selfaugment.SelfAugmentStateSnapshot) error {
	return testSnapshotStore().Write(dir, key, snapshot)
}
func testDocsValidator() qagate.Validator {
	return qagate.Validator{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List, ListSkills: install.ListSkillNames}
}
func ValidateQAGate(root string) selfverify.StepResult { return testDocsValidator().Validate(root) }

func ValidateMermaidDocs(root string) []string { return testDocsValidator().MermaidDocs(root) }

func readProbeState(dir, key string) (statecontract.StateResult, error) {
	service := stateapp.NewService(stateapp.Dependencies{StateDir: func() string { return dir }, StatePath: statepath.Path, OpenStore: func(dir string) (stateport.Store, error) { return sqlstore.Open(dir) }, ExistingRecords: statestore.ExistingRecords{}})
	return service.Read(key)
}

func ValidateRedactionAudit(root string) selfverify.StepResult {
	return testDocsValidator().RedactionAudit(root)
}
