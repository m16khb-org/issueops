// Package issueopsdecision is the application service that appends decisions to an
// issueops lifecycle record under holder authorization.
package issueopsdecision

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"
	"time"
)

// Repository persists a lifecycle record update. Update applies the mutation to the
// record with the given state root and id and stores the result.
type Repository interface {
	Update(
		context.Context,
		string,
		string,
		func(issueopscontract.Record) (issueopscontract.Record, error),
	) (issueopscontract.Record, error)
}

type clock interface {
	Now() time.Time
}

type pathMatcher interface {
	Same(string, string) bool
}
