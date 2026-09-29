package probe

import (
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification/probe/qagate"
	"issueops/internal/adapter/verification/probe/stateroundtrip"
	"issueops/internal/adapter/verification/probe/stepbudget"
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
func ValidateStateRoundtrip(binary, root string, seed int64) StepResult {
	return (stateroundtrip.Validator{StateRead: readProbeState, WriteRecord: statestore.WriteStateRecord, WriteSnapshot: testSnapshotStore().Write, OpenDatabase: func(dir string) (stateroundtrip.StateDatabase, error) { return sqlstore.Open(dir) }}).Validate(binary, root, seed)
}
func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return ValidateStateRoundtrip(binary, root, seed)
}
func ValidateStepBudgetBaseline(binary, root string, seed int64) StepResult {
	return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, stepbudget.StepBudgetValidationDeps{WriteSnapshot: testSnapshotStore().Write})
}
func writeSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return testSnapshotStore().Write(dir, key, snapshot)
}
func testDocsValidator() qagate.Validator {
	return qagate.Validator{ListDocs: docs.ListDocs, ListSkills: install.ListSkillNames}
}
func ValidateQAGate(root string) StepResult    { return testDocsValidator().Validate(root) }
func validateQAGate(root string) StepResult    { return ValidateQAGate(root) }
func ValidateMermaidDocs(root string) []string { return testDocsValidator().MermaidDocs(root) }
func validateMermaidDocs(root string) []string { return ValidateMermaidDocs(root) }

func readProbeState(dir, key string) (statecontract.StateResult, error) {
	service := stateapp.NewService(stateapp.Dependencies{StateDir: func() string { return dir }, StatePath: statepath.Path, OpenStore: func(dir string) (stateport.Store, error) { return sqlstore.Open(dir) }, ExistingRecords: statestore.ExistingRecords{}})
	return service.Read(key)
}

func ValidateRedactionAudit(root string) StepResult { return testDocsValidator().RedactionAudit(root) }
func validateRedactionAudit(root string) StepResult { return ValidateRedactionAudit(root) }
