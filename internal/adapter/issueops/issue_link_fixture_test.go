package issueops

import (
	"context"
	"time"

	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func issueLinkerForTest(root string) branchapp.Linker {
	return branchapp.Linker{Records: CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, liveTestVerifier()), Now: time.Now}
}
func LinkIssueOpsChild(root, id, childURL, title string) (model.IssueOpsRecord, error) {
	return issueLinkerForTest(root).Child(context.Background(), id, childURL, title, nil)
}
func LinkIssueOpsIssue(root, id, issueURL string) (model.IssueOpsRecord, error) {
	return issueLinkerForTest(root).Issue(context.Background(), id, issueURL, nil)
}
