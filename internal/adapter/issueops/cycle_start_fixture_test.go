package issueops

import (
	"context"
	app "issueops/internal/application/issueopsbranch"
	contract "issueops/internal/contract/issueops"
	"time"
)

func startIssueOpsFixture(stateRoot string, req contract.StartRequest) (contract.Record, error) {
	return (app.Starter{Records: CycleRecordStore{StateRoot: stateRoot}, Retired: CycleRecordStore{StateRoot: stateRoot}, Identity: CycleStartIdentity{RunGit: GitCmd}, Now: time.Now}).Start(context.Background(), req)
}
