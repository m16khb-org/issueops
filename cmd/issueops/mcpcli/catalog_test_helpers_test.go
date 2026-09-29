package mcpcli

import (
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"issueops/cmd/issueops/contractcli"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	mcpcontract "issueops/internal/contract/mcp"
)

func testMCPCatalog() mcpcontract.Catalog { return mcpcatalog.Build() }

func testHandleToolCall(params json.RawMessage) (any, *jsonrpc.Error) {
	return HandleToolCallWithDependencies(params, MCPDependencies{Catalog: testMCPCatalog()})
}

func init() {
	CompatibilityContract = func() any {
		return contractcli.BuildCompatibilityContract(clicatalog.Commands(), testMCPCatalog().Tools)
	}
}
