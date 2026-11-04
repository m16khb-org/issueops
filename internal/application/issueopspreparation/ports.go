package issueopspreparation

import (
	"context"
	"time"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

// Repository is the storage port of the preparation use case: it loads the
// cycle snapshot, guards workspace root and actor uniqueness, and commits the
// prepared execution.
type Repository interface {
	Load(context.Context, string) (preparationcontract.Snapshot, error)
	EnsureRootUnclaimed(context.Context, string, string) error
	// EnsureActorHoldsNoLease refuses an actor that already holds another
	// cycle's lease: a session holds one lease at a time.
	EnsureActorHoldsNoLease(context.Context, leasecontract.Actor) error
	CommitDirect(context.Context, DirectCommit) (preparationcontract.Result, error)
}

type clock interface{ Now() time.Time }

type directWorkspace interface {
	ProbeAccess(context.Context, preparationcontract.WorkspaceRequest, string) (preparationcontract.AccessResult, error)
	Prepare(context.Context, preparationcontract.WorkspaceRequest) (preparationcontract.WorkspaceReceipt, error)
}

type preparationEvidence interface {
	Workspace(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error)
	MaterializeDirect(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error
}

// DirectCommit carries everything CommitDirect persists for a confirmed
// preparation: the snapshot it was derived from, the command, the provisioned
// workspace, the artifact directory, and the link and claim timestamps.
type DirectCommit struct {
	Snapshot    preparationcontract.Snapshot
	Command     preparationcontract.Command
	Workspace   preparationcontract.WorkspaceReceipt
	ArtifactDir string
	LinkedAt    string
	ClaimedAt   string
}
