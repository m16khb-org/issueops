package issueopsdecision

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"
	"time"
)

type Repository interface {
	Update(
		context.Context,
		string,
		string,
		func(issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, error),
	) (issueopscontract.IssueOpsRecord, error)
}

type Clock interface {
	Now() time.Time
}

type PathMatcher interface {
	Same(string, string) bool
}
