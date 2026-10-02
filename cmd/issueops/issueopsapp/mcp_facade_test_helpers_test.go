package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
)

func serveMCPStreamContext(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
	return mcpcli.ServeMCPStreamContextWithDependencies(ctx, input, output, diagnostics, issueOpsMCPDependencies())
}

func mcpResources() []map[string]any {
	return mcpcatalog.Build().Resources
}

func callSDKTool(t *testing.T, params json.RawMessage, deps mcpcli.MCPDependencies) (any, *jsonrpc.Error) {
	t.Helper()
	var call mcp.CallToolParams
	if err := json.Unmarshal(params, &call); err != nil {
		t.Fatal(err)
	}
	session := startHistoryMCPTestSession(t, deps)
	result, err := session.CallTool(t.Context(), &call)
	if err != nil {
		var protocolErr *jsonrpc.Error
		if !errors.As(err, &protocolErr) {
			t.Fatalf("SDK call returned non-protocol error: %v", err)
		}
		return nil, protocolErr
	}
	content := make([]map[string]any, 0, len(result.Content))
	for _, item := range result.Content {
		text, ok := item.(*mcp.TextContent)
		if !ok {
			t.Fatalf("unexpected SDK content: %T", item)
		}
		content = append(content, map[string]any{"type": "text", "text": text.Text})
	}
	envelope := map[string]any{"content": content}
	if result.IsError {
		envelope["isError"] = true
	}
	return envelope, nil
}

func handleResourceRead(params json.RawMessage) (any, *jsonrpc.Error) {
	return mcpcli.HandleResourceRead(params, issueOpsMCPDependencies().Resources)
}

func textResult(text string) map[string]any {
	return mcpcli.TextResult(text)
}
