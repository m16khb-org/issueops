package mcpcli

import "context"

func testHandleLoopMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleLoopMCPToolCall(context.Background(), call, testLoopService())
}
