package issueopsapp

import (
	"context"
	adapter "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopsowner"
	model "issueops/internal/contract/issueops"
)

func newIssueOpsOwnerContext(stateRoot string) app.Service {
	return app.Service{Files: adapter.OwnerContextFiles{StateRoot: stateRoot}}
}

func issueOpsExecutionStatusHandler(_ context.Context, stateRoot, id string) (model.ExecutionResult, error) {
	return (app.ExecutionStatus{ReadRecord: (adapter.CycleRecordStore{StateRoot: stateRoot}).Load}).Read(id)
}
