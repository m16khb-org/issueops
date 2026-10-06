package mcpcli

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	issueops "issueops/internal/contract/issueops"
	mcpcontract "issueops/internal/contract/mcp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// If either transport ignores its catalog or validates after dispatch, invalid
// input reaches Release. Different schemas also detect a shared catalog cache.
func TestMCPCatalogValidationIsInstanceScopedAndPrecedesEffects(t *testing.T) {
	for _, entry := range []string{"stdio", "stream_conn"} {
		t.Run(entry, func(t *testing.T) {
			var calls [2]atomic.Int32
			deps := [2]MCPDependencies{}
			for i, kind := range []string{"string", "integer"} {
				deps[i] = MCPDependencies{Execution: testExecutionDeps(),
					Catalog: catalogForReleaseTest(kind),
					Release: func(_ context.Context, _ string, request issueops.ExecutionReleaseRequest) (issueops.ExecutionResult, error) {
						calls[i].Add(1)
						return issueops.ExecutionResult{OK: true, ID: request.ID}, nil
					},
				}
			}
			sessions := [2]*mcp.ClientSession{
				startMCPTransportTestSession(t, entry, deps[0]),
				startMCPTransportTestSession(t, entry, deps[1]),
			}
			invoke := func(i int, raw json.RawMessage) (bool, string) {
				result, err := sessions[i].CallTool(t.Context(), &mcp.CallToolParams{Name: "issueops_execution", Arguments: mustCatalogArgs(t, raw)})
				bytes, _ := json.Marshal(result)
				return err != nil, string(bytes)
			}
			input := []json.RawMessage{
				json.RawMessage(`{"action":"release","id":"io-string","generation":1,"proof":"value"}`),
				json.RawMessage(`{"action":"release","id":"io-integer","generation":1,"proof":3}`),
			}
			for i := range deps {
				rejected, _ := invoke(i, input[1-i])
				if !rejected || calls[i].Load() != 0 {
					t.Fatalf("instance %d accepted invalid schema or invoked effect: rejected=%v calls=%d", i, rejected, calls[i].Load())
				}
				rejected, result := invoke(i, input[i])
				want := []string{"io-string", "io-integer"}[i]
				if rejected || calls[i].Load() != 1 || !strings.Contains(result, want) {
					t.Fatalf("instance %d did not use its catalog/handler: rejected=%v calls=%d result=%s", i, rejected, calls[i].Load(), result)
				}
			}
		})
	}
}

func TestMCPServersAdvertiseTheirSuppliedCatalogs(t *testing.T) {
	for _, kind := range []string{"string", "integer"} {
		deps := MCPDependencies{Catalog: catalogForReleaseTest(kind)}
		session := startMCPTransportTestSession(t, "stdio", deps)
		tools, err := session.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(tools.Tools) != 1 || tools.Tools[0].Name != "issueops_execution" {
			t.Fatalf("unexpected tools: %#v", tools.Tools)
		}
		raw, err := json.Marshal(tools.Tools[0].InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]struct{ Type string }
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		if schema.Properties["proof"].Type != kind {
			t.Fatalf("schema from another server: %s", raw)
		}
		resources, err := session.ListResources(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(resources.Resources) != 1 || resources.Resources[0].URI != "issueops://test/"+kind {
			t.Fatalf("unexpected resources: %#v", resources.Resources)
		}
	}
}

func catalogForReleaseTest(kind string) mcpcontract.Catalog {
	return mcpcontract.Catalog{
		Tools: []map[string]any{{"name": "issueops_execution", "description": "release fixture", "inputSchema": map[string]any{
			"type": "object", "required": []string{"action", "id", "generation", "proof"},
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"release"}},
				"id":     map[string]any{"type": "string"}, "generation": map[string]any{"type": "integer"},
				"proof": map[string]any{"type": kind},
			},
		}}},
		Resources: []map[string]any{{"uri": "issueops://test/" + kind, "name": kind, "description": "catalog fixture", "mimeType": "application/json"}},
		Dispatch:  map[string]mcpcontract.DispatchGroup{"issueops_execution": mcpcontract.DispatchIssueOps},
	}
}

func mustCatalogArgs(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	return args
}
