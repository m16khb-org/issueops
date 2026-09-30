package issueopsapp

import (
	"context"

	model "issueops/internal/contract/issueops"
)

func LinkIssueOpsIssueForTest(root, id, issueURL string) (model.IssueOpsRecord, error) {
	return newIssueLinker(root).Issue(context.Background(), id, issueURL, nil)
}
