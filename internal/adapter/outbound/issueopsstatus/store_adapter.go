// Package issueopsstatus is the outbound adapter that serves issueops status
// reads from the issueops record store.
package issueopsstatus

import (
	"context"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
)

// Repository adapts an issueopsrecord.Store to the status read port.
type Repository struct {
	Store issueopsrecord.Store
}

// Read returns the status record of cycle id under stateRoot from the
// underlying record store.
func (repository Repository) Read(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsstatuscontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}
