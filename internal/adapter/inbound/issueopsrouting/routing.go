// Package issueopsrouting exposes the routing application service as a set of
// handler functions (Handlers) for the CLI and MCP layers; it is an inbound adapter.
package issueopsrouting

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"

	issueopsroutingapplication "issueops/internal/application/issueopsrouting"
	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
)

// Handlers bundles the routing record and score calls as function fields for
// the CLI/MCP layers. Record stores the skill chosen for a phase; Score compares
// recorded choices with the expected ones.
type Handlers struct {
	Record func(
		string,
		string,
		string,
		string,
		issueopscontract.Actor,
	) (issueopsroutingcontract.Record, error)
	Score func(
		string,
		string,
		[]issueopsroutingcontract.Expected,
	) (issueopsroutingcontract.Result, int, error)
}

// NewHandlers wires the routing application service into Handlers. Every call
// runs the service under context.Background().
func NewHandlers(service *issueopsroutingapplication.Service) Handlers {
	return Handlers{
		Record: func(
			stateRoot string,
			id string,
			phase string,
			skill string,
			actor issueopscontract.Actor,
		) (issueopsroutingcontract.Record, error) {
			return service.Record(context.Background(), stateRoot, id, phase, skill, actor)
		},
		Score: func(
			stateRoot string,
			id string,
			expected []issueopsroutingcontract.Expected,
		) (issueopsroutingcontract.Result, int, error) {
			return service.Score(context.Background(), stateRoot, id, expected)
		},
	}
}
