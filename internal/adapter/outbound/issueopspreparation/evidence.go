package issueopspreparation

import (
	"context"
	"fmt"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

// EvidenceDependencies are the injected hooks Evidence delegates to: resolving
// the workspace request for a snapshot and materializing staged artifacts.
type EvidenceDependencies struct {
	Workspace         func(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error)
	MaterializeDirect func(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error
}

// Evidence adapts the preparation use case's evidence needs (workspace
// request, staged artifact materialization) to injected functions.
type Evidence struct {
	dependencies EvidenceDependencies
}

// NewEvidence returns an Evidence that calls the functions in dependencies.
func NewEvidence(dependencies EvidenceDependencies) *Evidence {
	return &Evidence{dependencies: dependencies}
}

// Workspace resolves the workspace request for snapshot using a clone of it,
// so the resolver cannot mutate the caller's snapshot. It errors when no
// resolver is configured.
func (evidence *Evidence) Workspace(snapshot preparationcontract.Snapshot, confirm bool) (preparationcontract.WorkspaceRequest, error) {
	if evidence.dependencies.Workspace == nil {
		return preparationcontract.WorkspaceRequest{}, fmt.Errorf("preparation workspace resolver is unavailable")
	}
	return evidence.dependencies.Workspace(snapshot.Clone(), confirm)
}

// MaterializeDirect writes the staged artifacts of a direct preparation for
// snapshot (cloned) and receipt. It errors when no materializer is configured.
func (evidence *Evidence) MaterializeDirect(ctx context.Context, snapshot preparationcontract.Snapshot, receipt preparationcontract.WorkspaceReceipt) error {
	if evidence.dependencies.MaterializeDirect == nil {
		return fmt.Errorf("staged artifact materializer is unavailable")
	}
	return evidence.dependencies.MaterializeDirect(ctx, snapshot.Clone(), receipt)
}
