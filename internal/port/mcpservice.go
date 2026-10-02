package port

import (
	"context"

	"issueops/internal/contract/mcpservice"
)

type MCPService interface {
	Start(context.Context) (mcpservice.Status, error)
	Stop(context.Context) (mcpservice.Status, error)
	Status(context.Context) (mcpservice.Status, error)
}
