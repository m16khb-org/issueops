package issueopsapp

import (
	adapter "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopsowner"
	executionissue "issueops/internal/contract/executionissue"
)

func newIssueOpsOwnerContext(stateRoot string, readIssue executionissue.ExecutionIssueSnapshotReadFunc) app.Service {
	return app.Service{Files: adapter.OwnerContextFiles{StateRoot: stateRoot}, ReadIssue: readIssue, Template: adapter.ExecutionOwnerPromptTemplate(), ReadRecord: (adapter.CycleRecordStore{StateRoot: stateRoot}).Load}
}
