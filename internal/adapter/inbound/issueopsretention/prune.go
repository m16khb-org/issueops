// Package issueopsretention adapts the issueops retention application service
// to the plain function handlers used by inbound CLI and MCP entry points.
package issueopsretention

import (
	"context"
	"time"

	issueopsretentionapplication "issueops/internal/application/issueopsretention"
	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
)

// NewPruneHandler adapts the retention service to a function that prunes
// cycle records older than maxAge under stateRoot; deletion only happens when
// confirm is true.
func NewPruneHandler(service *issueopsretentionapplication.Service) func(
	string,
	time.Duration,
	bool,
) (issueopsretentioncontract.Result, error) {
	return func(stateRoot string, maxAge time.Duration, confirm bool) (issueopsretentioncontract.Result, error) {
		return service.Prune(context.Background(), stateRoot, maxAge, confirm)
	}
}
