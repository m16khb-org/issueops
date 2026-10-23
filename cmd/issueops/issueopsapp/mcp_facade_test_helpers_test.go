package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"issueops/cmd/issueops/mcpcli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
)

// toolRefusal reports whether an SDK tool call was refused and the text the
// model sees: a protocol error's message and data, or an isError result's
// text (tool failures are isError results since MCP 2025-11-25).
func toolRefusal(result *mcp.CallToolResult, err error) (string, bool) {
	var protocolErr *jsonrpc.Error
	if errors.As(err, &protocolErr) {
		return protocolErr.Message + " " + string(protocolErr.Data), true
	}
	if err != nil {
		return err.Error(), true
	}
	if result == nil || !result.IsError {
		return "", false
	}
	text := ""
	for _, item := range result.Content {
		if content, ok := item.(*mcp.TextContent); ok {
			text += content.Text
		}
	}
	return text, true
}

// envelopeRefusal is toolRefusal for callSDKTool's envelope.
func envelopeRefusal(envelope any, err *jsonrpc.Error) (string, bool) {
	if err != nil {
		return err.Message + " " + string(err.Data), true
	}
	result, _ := envelope.(map[string]any)
	if isError, _ := result["isError"].(bool); !isError {
		return "", false
	}
	text := ""
	content, _ := result["content"].([]map[string]any)
	for _, item := range content {
		value, _ := item["text"].(string)
		text += value
	}
	return text, true
}

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
