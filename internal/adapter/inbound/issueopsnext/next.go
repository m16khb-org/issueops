// Package issueopsnext adapts the issueops next-stage application service to
// the plain function handlers used by inbound CLI and MCP entry points.
package issueopsnext

import (
	"context"

	issueopsnextapplication "issueops/internal/application/issueopsnext"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

// NewNextHandler adapts the next-stage service to a function that resolves the
// next issueops stage for the cycle id, using stateRoot and cwd for lookup.
func NewNextHandler(service *issueopsnextapplication.Service) func(
	string,
	string,
	string,
) (issueopsnextcontract.Result, error) {
	return func(stateRoot, cwd, id string) (issueopsnextcontract.Result, error) {
		return service.Next(context.Background(), stateRoot, cwd, id)
	}
}
