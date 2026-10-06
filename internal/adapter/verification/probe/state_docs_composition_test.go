package probe

import (
	"context"
	selfverify "issueops/internal/contract/selfverify"

	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification/probe/qagate"
	"issueops/internal/adapter/verification/probe/stepbudget"
	docsapp "issueops/internal/application/docs"
	app "issueops/internal/application/selfaugment"
	statecontract "issueops/internal/contract/state"
	"time"
)

func testSnapshotStore() app.SnapshotStore {
	return app.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: func(dir, key string, record statecontract.RecordEnvelope) (string, error) {
		return statestore.WriteStateRecord(context.Background(), dir, key, record)
	}, Now: time.Now}
}

func ValidateStepBudgetBaseline(binary, root string, seed int64) selfverify.StepResult {
	return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, stepbudget.StepBudgetValidationDeps{WriteSnapshot: testSnapshotStore().Write})
}
func testDocsValidator() qagate.Validator {
	return qagate.Validator{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List, ListSkills: install.ListSkillNames}
}
func ValidateQAGate(root string) selfverify.StepResult { return testDocsValidator().Validate(root) }

func ValidateRedactionAudit(root string) selfverify.StepResult {
	return testDocsValidator().RedactionAudit(root)
}
