package issueopscompletion

import (
	"context"
	"time"

	completioncontract "issueops/internal/contract/issueopscompletion"
)

type RecordTransition func(completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error)

type RepositoryResult struct {
	Record    completioncontract.RecordSnapshot
	Execution completioncontract.Execution
}

type Repository interface {
	Update(context.Context, string, RecordTransition) (RepositoryResult, error)
}

type Environment interface {
	PathsMatch(string, string) bool
	CurrentHead(context.Context, string) (string, error)
	VerifyReport(string, string) (string, error)
}

type Clock interface{ Now() time.Time }

// ActorVerifier proves the caller (native ancestry or a bound capability) and
// returns its verified identity. It never decides holder or generation.
type ActorVerifier func(context.Context, completioncontract.Actor, []completioncontract.ProcessReceipt) (completioncontract.Actor, error)
