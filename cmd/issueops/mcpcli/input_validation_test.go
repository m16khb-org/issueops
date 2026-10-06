package mcpcli

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	mcpcontract "issueops/internal/contract/mcp"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func inputValidationCatalog() mcpcontract.Catalog {
	return mcpcontract.Catalog{Tools: []map[string]any{{
		"name": "probe",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"nested": map[string]any{
					"type":       "object",
					"properties": map[string]any{"value": map[string]any{"type": "integer"}},
				},
			},
		},
	}}}
}

func TestSDKInputSchemaErrorsPrecedeEffects(t *testing.T) {
	for _, test := range []struct {
		name    string
		catalog mcpcontract.Catalog
		code    int64
	}{
		{"unknown", mcpcontract.Catalog{}, -32602},
		{"missing-schema", mcpcontract.Catalog{Tools: []map[string]any{{"name": "probe"}}}, -32603},
		{"unsupported-schema", mcpcontract.Catalog{Tools: []map[string]any{{"name": "probe", "inputSchema": map[string]any{"$ref": "#/missing"}}}}, -32603},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := sdkToolHandlerWithContext(test.catalog, func(context.Context, MCPToolCall) MCPToolOutcome {
				t.Fatal("invalid schema reached effect")
				return MCPToolOutcome{}
			}, "probe")
			_, err := handler(t.Context(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)}})
			var protocol *jsonrpc.Error
			if !errors.As(err, &protocol) || protocol.Code != test.code {
				t.Fatalf("error=%v, want protocol code %d", err, test.code)
			}
		})
	}
}

func BenchmarkSDKToolHandler(b *testing.B) {
	handler := sdkToolHandlerWithContext(inputValidationCatalog(), func(context.Context, MCPToolCall) MCPToolOutcome {
		return mcpToolPayload(map[string]any{"ok": true})
	}, "probe")
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"nested":{"value":1}}`)}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := handler(b.Context(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSDKInputValidationParallelNestedProperties(t *testing.T) {
	var calls atomic.Int32
	handler := sdkToolHandlerWithContext(inputValidationCatalog(), func(context.Context, MCPToolCall) MCPToolOutcome {
		calls.Add(1)
		return mcpToolPayload(map[string]any{"ok": true})
	}, "probe")
	t.Run("requests", func(t *testing.T) {
		for _, raw := range []string{`{"nested":{"value":1}}`, `{"nested":{"extra":1}}`, `{"nested":{"value":"wrong"}}`} {
			t.Run(raw, func(t *testing.T) {
				t.Parallel()
				req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(raw)}}
				_, err := handler(t.Context(), req)
				if (err == nil) != (raw == `{"nested":{"value":1}}`) {
					t.Fatalf("unexpected validation result: %v", err)
				}
			})
		}
	})
	if calls.Load() != 1 {
		t.Fatalf("invalid requests reached effect: calls=%d", calls.Load())
	}
}
