package issueopsdecision

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"

	"issueops/internal/adapter/outbound/issueopsrecord"
)

type Repository struct {
	Store issueopsrecord.Store
}

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
