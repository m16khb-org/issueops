// Package issueopsartifact adapts the artifact staging application service to
// the function-valued handlers the CLI and MCP layers call. It is an inbound adapter.
package issueopsartifact

import (
	"context"

	issueopsartifactapplication "issueops/internal/application/issueopsartifact"
	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
)

// Handlers exposes artifact stage, list, read, and unstage operations as
// functions keyed by state root and cycle ID.
type Handlers struct {
	Stage   func(string, string, string, []byte) (issueopsartifactcontract.Record, error)
	Names   func(string, string) ([]string, error)
	Staged  func(string, string) (issueopsartifactcontract.Staged, error)
	Unstage func(string, string, string) (issueopsartifactcontract.Record, error)
}

// NewHandlers binds each handler to service, calling it with a background
// context.
func NewHandlers(service *issueopsartifactapplication.Service) Handlers {
	return Handlers{
		Stage: func(
			stateRoot string,
			id string,
			name string,
			content []byte,
		) (issueopsartifactcontract.Record, error) {
			return service.Stage(context.Background(), stateRoot, id, name, content)
		},
		Names: func(stateRoot, id string) ([]string, error) {
			return service.Names(context.Background(), stateRoot, id)
		},
		Staged: func(stateRoot, id string) (issueopsartifactcontract.Staged, error) {
			return service.Staged(context.Background(), stateRoot, id)
		},
		Unstage: func(
			stateRoot string,
			id string,
			name string,
		) (issueopsartifactcontract.Record, error) {
			return service.Unstage(context.Background(), stateRoot, id, name)
		},
	}
}
