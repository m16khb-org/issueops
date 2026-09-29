package mcpcli

import (
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"issueops/cmd/issueops/contractcli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	mcpcontract "issueops/internal/contract/mcp"
)

func testMCPCatalog() mcpcontract.Catalog { return mcpcatalog.Build() }

func testHandleToolCall(params json.RawMessage) (any, *jsonrpc.Error) {
	return HandleToolCallWithDependencies(params, MCPDependencies{Catalog: testMCPCatalog()})
}

func init() {
	contractcli.MCPTools = func() []map[string]any { return testMCPCatalog().Tools }
	CompatibilityContract = func() any { return contractcli.BuildCompatibilityContract() }
}
