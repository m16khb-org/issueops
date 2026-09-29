package issueopsapp

import (
	"context"
	adapter "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopsbranch"
	contract "issueops/internal/contract/issueops"
	"time"
)

func startIssueOpsFixture(stateRoot string, req contract.IssueOpsStartRequest) (contract.IssueOpsRecord, error) {
	return (app.Starter{Records: adapter.CycleRecordStore{StateRoot: stateRoot}, Identity: adapter.CycleStartIdentity{}, Now: time.Now}).Start(context.Background(), req)
}
