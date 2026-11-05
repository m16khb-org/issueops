package issueopsdecision

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"

	"issueops/internal/adapter/outbound/issueopsrecord"
)

// Repository adapts the issueops record Store to the decision service's
// repository port.
type Repository struct {
	Store issueopsrecord.Store
}

// Update applies mutate to the record with id under the store's update guard. The
// mutation is persisted only when mutate returns no error.
func (repository Repository) Update(
	ctx context.Context,
	stateRoot string,
	id string,
	mutate func(issueopscontract.Record) (issueopscontract.Record, error),
) (issueopscontract.Record, error) {
	return repository.Store.Update(
		ctx,
		stateRoot,
		id,
		func(record issueopscontract.Record) (issueopscontract.Record, bool, error) {
			record, err := mutate(record)
			return record, err == nil, err
		},
	)
}
