// Package issueopsdecision adapts the decision recording application service
// to the function-valued handlers the CLI and MCP layers call. It is an inbound adapter.
package issueopsdecision

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"

	issueopsdecisionapplication "issueops/internal/application/issueopsdecision"
	issueopsdecisioncontract "issueops/internal/contract/issueopsdecision"
)

// Handlers exposes decision recording with and without a native actor. Add
// records without actor verification; AddWithActor passes the actor to the service.
type Handlers struct {
	Add func(
		string,
		string,
		issueopsdecisioncontract.Request,
	) (issueopscontract.Record, error)
	AddWithActor func(
		string,
		string,
		issueopsdecisioncontract.Request,
		issueopscontract.Actor,
	) (issueopscontract.Record, error)
}

// NewHandlers binds Add and AddWithActor to service, calling it with a
// background context.
func NewHandlers(service *issueopsdecisionapplication.Service) Handlers {
	return Handlers{
		Add: func(
			stateRoot string,
			id string,
			request issueopsdecisioncontract.Request,
		) (issueopscontract.Record, error) {
			return service.Add(context.Background(), stateRoot, id, request, nil)
		},
		AddWithActor: func(
			stateRoot string,
			id string,
			request issueopsdecisioncontract.Request,
			actor issueopscontract.Actor,
		) (issueopscontract.Record, error) {
			return service.Add(context.Background(), stateRoot, id, request, &actor)
		},
	}
}
