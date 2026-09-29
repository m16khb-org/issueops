package mcpcli

import (
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"issueops/cmd/issueops/contractcli"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
	mcpcontract "issueops/internal/contract/mcp"
)

func testMCPCatalog() mcpcontract.Catalog { return mcpcatalog.Build() }

func testHandleToolCall(params json.RawMessage) (any, *jsonrpc.Error) {
	return HandleToolCallWithDependencies(params, MCPDependencies{Catalog: testMCPCatalog(), SelfHistory: historyServiceForTest()})
}

func init() {
	CompatibilityContract = func() any {
		return contractcli.BuildCompatibilityContract(clicatalog.Commands(), testMCPCatalog().Tools)
	}
}

func historyServiceForTest() augmentapp.HistoryService {
	return augmentapp.HistoryService{StateDir: statestore.StateDir, List: statestore.StateList, Read: statestore.StateRead, Delete: statestore.StateDelete}
}
func testHandleSelfLoopMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleSelfLoopMCPToolCall(call, historyServiceForTest())
}
