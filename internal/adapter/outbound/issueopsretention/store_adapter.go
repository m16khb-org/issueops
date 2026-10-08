package issueopsretention

import (
	"context"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
	"issueops/internal/port"
)

const artifactStageBucket = port.ArtifactStageBucket

type Repository struct {
	Store issueopsrecord.Store
}

func (repository Repository) ListIDs(ctx context.Context, stateRoot string) ([]string, error) {
	return repository.Store.ListIDs(ctx, stateRoot)
}

func (repository Repository) ReadUnchecked(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsretentioncontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}

func (repository Repository) DeleteIfUnchanged(
	ctx context.Context,
	stateRoot string,
	id string,
	record issueopsretentioncontract.Record,
) error {
	return repository.Store.DeleteIfUnchanged(ctx, stateRoot, id, record, artifactStageBucket)
}
