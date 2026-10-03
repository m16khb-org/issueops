package mcpcli

import "context"

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
