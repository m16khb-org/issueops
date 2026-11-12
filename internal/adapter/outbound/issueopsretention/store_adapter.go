package issueopsretention

import (
	"context"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
	"issueops/internal/port"
)

const artifactStageBucket = port.ArtifactStageBucket

// Repository adapts an issueopsrecord.Store to the retention use case's repository port.
type Repository struct {
	Store issueopsrecord.Store
}

// ListIDs returns the IDs of every stored record under stateRoot.
func (repository Repository) ListIDs(ctx context.Context, stateRoot string) ([]string, error) {
	return repository.Store.ListIDs(ctx, stateRoot)
}

// ReadUnchecked reads the stored record id from stateRoot without any retention-specific validation.
func (repository Repository) ReadUnchecked(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsretentioncontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}

// DeleteIfUnchanged deletes record id, and its staged artifact bucket, only if the stored
// record still equals record, so a concurrent update is never removed. It stores outcome
// as the retained row in the same commit.
func (repository Repository) DeleteIfUnchanged(
	ctx context.Context,
	stateRoot string,
	id string,
	record issueopsretentioncontract.Record,
	outcome issueopsretentioncontract.CycleOutcome,
) error {
	data, err := issueopscontract.EncodeCycleOutcome(id, outcome)
	if err != nil {
		return err
	}
	row := port.RecordMutation{Bucket: port.CycleOutcomeBucket, ID: id, Data: data}
	return repository.Store.DeleteIfUnchanged(ctx, stateRoot, id, record, []port.RecordMutation{row}, artifactStageBucket)
}
