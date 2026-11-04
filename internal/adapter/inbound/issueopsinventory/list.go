// Package issueopsinventory adapts the issueops inventory application service
// to the plain function handlers used by inbound CLI and MCP entry points.
package issueopsinventory

import (
	"context"

	issueopsapplication "issueops/internal/application/issueopsinventory"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

// NewListHandler adapts the inventory service to a function that lists the
// issueops cycles recorded under stateRoot for the given repo.
func NewListHandler(service *issueopsapplication.Service) func(
	string,
	string,
) (issueopsinventorycontract.ListResult, error) {
	return func(stateRoot, repo string) (issueopsinventorycontract.ListResult, error) {
		return service.ListCycles(context.Background(), stateRoot, repo)
	}
}
