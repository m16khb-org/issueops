// Package issueopsretention prunes issueops state records older than a maximum
// age. It is an application-layer service that reaches storage through Repository.
package issueopsretention

import (
	"context"
	"time"

	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
)

// Repository is the state store view retention needs: list record IDs, read a
// record without validating it, and delete a record only if it is unchanged
// since it was read, storing outcome in the same commit.
type Repository interface {
	ListIDs(context.Context, string) ([]string, error)
	ReadUnchecked(context.Context, string, string) (issueopsretentioncontract.Record, error)
	DeleteIfUnchanged(context.Context, string, string, issueopsretentioncontract.Record, issueopsretentioncontract.CycleOutcome) error
}

type clock interface {
	Now() time.Time
}
