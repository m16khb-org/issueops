package issueopspreparation

import (
	"context"
	"fmt"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type EvidenceDependencies struct {
	Workspace         func(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error)
	MaterializeDirect func(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error
}

type Evidence struct {
	dependencies EvidenceDependencies
}

func NewEvidence(dependencies EvidenceDependencies) *Evidence {
	return &Evidence{dependencies: dependencies}
}

func (evidence *Evidence) Workspace(snapshot preparationcontract.Snapshot, confirm bool) (preparationcontract.WorkspaceRequest, error) {
	if evidence.dependencies.Workspace == nil {
		return preparationcontract.WorkspaceRequest{}, fmt.Errorf("preparation workspace resolver is unavailable")
	}
	return evidence.dependencies.Workspace(snapshot.Clone(), confirm)
}

func (evidence *Evidence) MaterializeDirect(ctx context.Context, snapshot preparationcontract.Snapshot, receipt preparationcontract.WorkspaceReceipt) error {
	if evidence.dependencies.MaterializeDirect == nil {
		return fmt.Errorf("staged artifact materializer is unavailable")
	}
	return evidence.dependencies.MaterializeDirect(ctx, snapshot.Clone(), receipt)
}
