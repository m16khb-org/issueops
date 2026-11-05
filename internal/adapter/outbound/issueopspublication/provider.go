// Package issueopspublication is the outbound adapter that bridges the publication
// application's Provider port to the issue-provider create and inventory
// functions.
package issueopspublication

import (
	"context"
	"errors"
	"fmt"

	application "issueops/internal/application/issueopspublication"
	contract "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

type providerCreateFunc func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, error)
type providerInspectFunc func(context.Context, contract.Intent) (contract.Inventory, bool, error)

// ProviderGateway implements the publication Provider port by calling injected
// provider create and inspect functions.
type ProviderGateway struct {
	create  providerCreateFunc
	inspect providerInspectFunc
}

// NewProviderGateway returns a ProviderGateway that creates through create and
// reads the remote inventory through inspect.
func NewProviderGateway(create providerCreateFunc, inspect providerInspectFunc) *ProviderGateway {
	return &ProviderGateway{create: create, inspect: inspect}
}

// Create creates the remote artifact through the provider bridge on a clone of
// request. The invocation state is NotInvokedProven only when the provider error
// proves the call never started; otherwise it is Unknown. A missing bridge is an
// error.
func (g *ProviderGateway) Create(ctx context.Context, provider string, request contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
	if g == nil || g.create == nil {
		return contract.ProviderCreateResult{}, contract.InvocationUnknown, fmt.Errorf("publication provider create bridge is required")
	}
	result, err := g.create(ctx, provider, request.Clone())
	invocation := contract.InvocationUnknown
	if typed, ok := errors.AsType[*port.IssueProviderCreateError](err); ok && !typed.Invoked {
		invocation = contract.InvocationNotInvokedProven
	}
	return result, invocation, err
}

// Inspect reads the remote inventory for intent through the provider bridge and
// returns a clone of it with whether a lookup was attempted. A missing bridge is
// an error.
func (g *ProviderGateway) Inspect(ctx context.Context, intent contract.Intent) (contract.Inventory, bool, error) {
	if g == nil || g.inspect == nil {
		return contract.Inventory{}, false, fmt.Errorf("publication provider inventory bridge is required")
	}
	inventory, attempted, err := g.inspect(ctx, intent.Clone())
	return inventory.Clone(), attempted, err
}

var _ application.Provider = (*ProviderGateway)(nil)
