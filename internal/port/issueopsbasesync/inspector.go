// Package issueopsbasesync defines the port that observes how far a worktree
// is from its base branch. Adapters implement it with git; the application
// layer depends only on this interface.
package issueopsbasesync

import "context"

// Request names the worktree to inspect and the base branch to compare against.
type Request struct {
	Worktree   string
	BaseBranch string
}

// Receipt is the observation of a worktree against its base: both commit IDs
// and whether the worktree must sync before it can proceed.
type Receipt struct {
	BaseOID      string
	WorkOID      string
	SyncRequired bool
}

// Inspector observes a worktree against its base branch without changing it.
type Inspector interface {
	Observe(context.Context, Request) (Receipt, error)
}
