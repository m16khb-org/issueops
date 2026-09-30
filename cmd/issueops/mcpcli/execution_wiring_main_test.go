package mcpcli

import (
	"context"
	issueopscore "issueops/internal/adapter/issueops"
)

// 프로덕션에서는 issueopsapp이 주입한다. 실행 CLI 테스트는 실제 액션 경로를
// 검증하므로 같은 배선을 재현한다.
func testExecutionDeps() ExecutionDeps {
	root := issueOpsStateRootForTest()
	return ExecutionDeps{ExecuteExecution: testExecutionService().Execute, ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry, IssueOpsStateRoot: func() string { return root }}
}

func handleMCPIssueOpsExecutionWithDependencies(args map[string]any, deps MCPDependencies) MCPToolOutcome {
	return handleMCPIssueOpsExecutionWithContext(context.Background(), args, deps)
}
