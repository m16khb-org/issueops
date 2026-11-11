// Package issueopsview adapts the issueops view application service to the
// plain function handler used by the inbound CLI entry point.
package issueopsview

import (
	"context"

	issueopsviewapplication "issueops/internal/application/issueopsview"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
)

// NewViewHandler adapts the view service to a function that renders view of
// the cycle id under stateRoot. cwd is the caller's working directory.
func NewViewHandler(service *issueopsviewapplication.Service) func(
	stateRoot, cwd, id, view string,
) (issueopsviewcontract.Result, error) {
	return func(stateRoot, cwd, id, view string) (issueopsviewcontract.Result, error) {
		return service.View(context.Background(), stateRoot, cwd, id, view)
	}
}
