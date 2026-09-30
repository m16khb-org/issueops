package issueopsapp

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"io"

	"issueops/cmd/issueops/mcpcli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
)

func serveMCPStreamContext(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
	return mcpcli.ServeMCPStreamContextWithDependencies(ctx, input, output, diagnostics, issueOpsMCPDependencies())
}

func mcpResources() []map[string]any {
	return mcpcatalog.Build().Resources
}

func handleToolCall(params json.RawMessage) (any, *jsonrpc.Error) {
	return mcpcli.HandleToolCallWithDependencies(params, issueOpsMCPDependencies())
}

func handleResourceRead(params json.RawMessage) (any, *jsonrpc.Error) {
	return mcpcli.HandleResourceRead(params, issueOpsMCPDependencies().Resources)
}

func textResult(text string) map[string]any {
	return mcpcli.TextResult(text)
}
