// Package issueopsartifact persists the staged plan and architecture artifacts
// of an issueops cycle. It is an outbound adapter over the issueops record store.
package issueopsartifact

import (
	"context"
	"encoding/json"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
	"issueops/internal/port"
)

const artifactStageBucket = port.ArtifactStageBucket

// Repository stores staged artifacts as a record-related row in the issueops
// record Store.
type Repository struct {
	Store issueopsrecord.Store
}

// Update loads the staged artifacts of cycle id, applies mutate to them, and
// stores the result atomically with the record. An empty staged set deletes
// the stored row. It returns the record.
func (repository Repository) Update(
	ctx context.Context,
	stateRoot string,
	id string,
	mutate func(
		issueopsartifactcontract.Record,
		issueopsartifactcontract.Staged,
	) (issueopsartifactcontract.Staged, error),
) (issueopsartifactcontract.Record, error) {
	return repository.Store.UpdateRelated(
		ctx,
		stateRoot,
		id,
		artifactStageBucket,
		func(
			record issueopsartifactcontract.Record,
			data []byte,
			found bool,
		) ([]byte, bool, error) {
			staged := issueopsartifactcontract.Staged{}
			if found {
				if err := json.Unmarshal(data, &staged); err != nil {
					return nil, false, err
				}
			}
			staged, err := mutate(record, staged)
			if err != nil {
				return nil, false, err
			}
			if len(staged) == 0 {
				return nil, true, nil
			}
			data, err = json.Marshal(staged)
			return data, false, err
		},
	)
}

// ReadStaged returns the staged artifacts of cycle id, or an empty set when
// nothing is staged.
func (repository Repository) ReadStaged(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsartifactcontract.Staged, error) {
	data, found, err := repository.Store.ReadRelated(
		ctx,
		stateRoot,
		id,
		artifactStageBucket,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return issueopsartifactcontract.Staged{}, nil
	}
	var staged issueopsartifactcontract.Staged
	if err := json.Unmarshal(data, &staged); err != nil {
		return nil, err
	}
	return staged, nil
}
