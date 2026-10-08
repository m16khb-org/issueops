package issueopsapp

import (
	"context"
	adapter "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopsowner"
	agentmodelcontract "issueops/internal/contract/agentmodel"
	executionissue "issueops/internal/contract/executionissue"
	model "issueops/internal/contract/issueops"
)

func newIssueOpsOwnerContext(stateRoot string, readIssue executionissue.ExecutionIssueSnapshotReadFunc) app.Service {
	return app.Service{Files: adapter.OwnerContextFiles{StateRoot: stateRoot}, ReadIssue: readIssue, Template: adapter.ExecutionOwnerPromptTemplate(), ReadRecord: (adapter.CycleRecordStore{StateRoot: stateRoot}).Load, ResolveModel: resolveOwnerRoleModel}
}

func resolveOwnerRoleModel(host string, role agentmodelcontract.Role, repo string) (string, string, error) {
	resolution, err := resolveAgentModel(context.Background(), host, role, "", repo)
	return resolution.Model, resolution.Effort, err
}

func issueOpsExecutionStatusHandler(_ context.Context, stateRoot, id string) (model.ExecutionResult, error) {
	return (app.ExecutionStatus{ReadRecord: (adapter.CycleRecordStore{StateRoot: stateRoot}).Load}).Read(id)
}
