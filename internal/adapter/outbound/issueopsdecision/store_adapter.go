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
	mutate func(issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, error),
) (issueopscontract.IssueOpsRecord, error) {
	return repository.Store.Update(
		ctx,
		stateRoot,
		id,
		func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
			record, err := mutate(record)
			return record, err == nil, err
		},
	)
}
