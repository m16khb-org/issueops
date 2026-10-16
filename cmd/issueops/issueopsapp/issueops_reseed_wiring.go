package issueopsapp

import (
	"context"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	basesyncoutbound "issueops/internal/adapter/outbound/issueopsbasesync"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	model "issueops/internal/contract/issueops"
	ownerdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

func issueOpsReseedHandler(ctx context.Context, stateRoot string, request model.ExecutionReseedRequest) (model.ExecutionReplaceResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return model.ExecutionReplaceResult{ID: request.ID, Action: model.ExecutionReplaceReseed}, err
	}
	fence, err := leaseoutbound.NewSQLiteReseedFence(stateRoot, func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
	if err != nil {
		return model.ExecutionReplaceResult{ID: request.ID, Action: model.ExecutionReplaceReseed}, err
	}
	inventory := leaseoutbound.NewReseedInventory(leaseoutbound.InspectNativeProcess)
	artifacts := leaseoutbound.NewReseedArtifacts()
	baseSync := basesyncoutbound.NewInspector(basesyncoutbound.RunGit)
	service := leaseapp.NewReseedService(fence, leaseoutbound.NewReseedRepository(db), inventory, baseSync, artifacts, leaseoutbound.UTCClock{}, issueOpsActorVerifier(), leaseoutbound.FilesystemPathMatcher{})
	return leaseinbound.NewReseedHandler(service, ownerdomain.ReplacementClaimCommand)(ctx, stateRoot, request)
}
