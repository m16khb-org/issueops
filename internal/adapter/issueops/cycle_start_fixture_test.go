package issueops

import (
	"context"
	app "issueops/internal/application/issueopsbranch"
	contract "issueops/internal/contract/issueops"
	"time"
)

func startIssueOpsFixture(stateRoot string, req contract.IssueOpsStartRequest) (contract.IssueOpsRecord, error) {
	return (app.Starter{Records: CycleRecordStore{StateRoot: stateRoot}, Identity: CycleStartIdentity{}, Now: time.Now}).Start(context.Background(), req)
}
