package mcpcli

func testHandleLoopMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleLoopMCPToolCall(call, testLoopService())
}
