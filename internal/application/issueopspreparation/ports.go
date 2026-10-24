package issueopspreparation

import (
	"context"
	"time"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type Repository interface {
	Load(context.Context, string) (preparationcontract.Snapshot, error)
	EnsureRootUnclaimed(context.Context, string, string) error
	// EnsureActorHoldsNoLease refuses an actor that already holds another
	// cycle's lease: a session holds one lease at a time.
	EnsureActorHoldsNoLease(context.Context, leasecontract.Actor) error
	CommitDirect(context.Context, DirectCommit) (preparationcontract.Result, error)
}

type Clock interface{ Now() time.Time }

type DirectWorkspace interface {
	ProbeAccess(context.Context, preparationcontract.WorkspaceRequest, string) (preparationcontract.AccessResult, error)
	Prepare(context.Context, preparationcontract.WorkspaceRequest) (preparationcontract.WorkspaceReceipt, error)
}

type PreparationEvidence interface {
	Workspace(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error)
	MaterializeDirect(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error
}

type DirectCommit struct {
	Snapshot    preparationcontract.Snapshot
	Command     preparationcontract.Command
	Workspace   preparationcontract.WorkspaceReceipt
	ArtifactDir string
	LinkedAt    string
	ClaimedAt   string
}
