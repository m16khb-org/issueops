package mcpcli

import "context"

func testHandleLoopMCPToolCall(call MCPToolCall) mcpToolOutcome {
	return handleLoopMCPToolCall(context.Background(), call, testLoopService())
}
