// Package issueopsstatus adapts the issueops status application service to
// the plain function handlers used by inbound CLI and MCP entry points.
package issueopsstatus

import (
	"context"

	issueopsstatusapplication "issueops/internal/application/issueopsstatus"
	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
)

// NewStatusHandler adapts the status service to a function that reads the
// status record of cycle id under stateRoot.
func NewStatusHandler(service *issueopsstatusapplication.Service) func(
	string,
	string,
) (issueopsstatuscontract.Record, error) {
	return func(stateRoot, id string) (issueopsstatuscontract.Record, error) {
		return service.Status(context.Background(), stateRoot, id)
	}
}
