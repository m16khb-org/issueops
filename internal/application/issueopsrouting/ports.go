package issueopsrouting

import (
	"context"
	"time"

	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
)

// Repository is the storage port for cycle records read and updated by the
// routing service. Update runs the callback against the stored record and
// persists the result only when the callback reports a change.
type Repository interface {
	Read(context.Context, string, string) (issueopsroutingcontract.Record, error)
	Update(
		context.Context,
		string,
		string,
		func(issueopsroutingcontract.Record) (issueopsroutingcontract.Record, bool, error),
	) (issueopsroutingcontract.Record, error)
}

type clock interface {
	Now() time.Time
}

type pathMatcher interface {
	Same(string, string) bool
}
