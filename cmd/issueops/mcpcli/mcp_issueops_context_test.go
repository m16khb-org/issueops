package mcpcli

import (
	"context"
	"errors"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestIssueOpsMCPExecutionPropagatesRequestCancellation(t *testing.T) {
	execution := testExecutionDeps()
	var observed context.Context
	execution.ExecuteExecution = func(
		ctx context.Context,
		_ string,
		_ issueopscontract.ExecutionActionRequest,
		_ port.ExecutionActionDependencies,
	) (any, error) {
		observed = ctx
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handleIssueOpsMCPToolCallWithContext(
		ctx,
		MCPToolCall{
			Name:      "issueops_execution",
			Arguments: map[string]any{"action": "status", "id": "io-context"},
		},
		MCPDependencies{Execution: execution, Catalog: mcpcatalog.Build()},
	)

	if observed == nil || !errors.Is(observed.Err(), context.Canceled) {
		t.Fatalf("execution context = %v, want canceled request context", observed)
	}
}
