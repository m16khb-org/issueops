package port

import (
	"context"

	"issueops/internal/contract/mcpservice"
)

// MCPService controls the per-user MCP service: Start and Stop change its state and each method
// returns the resulting Status.
type MCPService interface {
	Start(context.Context) (mcpservice.Status, error)
	Stop(context.Context) (mcpservice.Status, error)
	Status(context.Context) (mcpservice.Status, error)
}
