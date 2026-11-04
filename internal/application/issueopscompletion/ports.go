package issueopscompletion

import (
	"context"
	"time"

	completioncontract "issueops/internal/contract/issueopscompletion"
)

// RecordTransition computes the next record from the current one. The bool reports whether the
// record changed and must be persisted; returning false leaves the stored record untouched.
type RecordTransition func(completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error)

// RepositoryResult is the state a Repository returns after Update: the stored record and its execution.
type RepositoryResult struct {
	Record    completioncontract.RecordSnapshot
	Execution completioncontract.Execution
}

// Repository persists cycle records for completion.
type Repository interface {
	Update(context.Context, string, RecordTransition) (RepositoryResult, error)
}

// Environment exposes the filesystem and git observations completion needs: canonical path comparison,
// the current HEAD of a worktree, verification report resolution, and changed-path listing.
type Environment interface {
	PathsMatch(string, string) bool
	CurrentHead(context.Context, string) (string, error)
	VerifyReport(string, string) (string, error)
	// ChangedPathsSince lists the root-relative paths that differ between from and to,
	// leaving out the verification report (a resolved path from VerifyReport).
	ChangedPathsSince(ctx context.Context, root, from, to, report string) ([]string, error)
}

type clock interface{ Now() time.Time }

// actorVerifier proves the caller (native ancestry or a bound capability) and
// returns its verified identity. It never decides holder or generation.
type actorVerifier func(context.Context, completioncontract.Actor, []completioncontract.ProcessReceipt) (completioncontract.Actor, error)
