package mcpcli

import "context"

func handleIssueOpsMCPToolCallWithDependencies(call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	return handleIssueOpsMCPToolCallWithContext(context.Background(), call, deps)
}

func handleIssueOpsMCPToolCallWithContext(
	ctx context.Context,
	call MCPToolCall,
	deps MCPDependencies,
) MCPToolOutcome {
	if call.Name == "issueops_execution" {
		return handleMCPIssueOpsExecutionWithContext(ctx, call.Arguments, deps)
	}
	return MCPToolOutcome{}
}
