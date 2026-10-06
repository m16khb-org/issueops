package mcpcli

import (
	adapter "issueops/internal/adapter/gates"
	policyadapter "issueops/internal/adapter/policy"
	app "issueops/internal/application/gates"
	policyapp "issueops/internal/application/policy"
)

func testGatesService() app.Service {
	policy := policyapp.Service{Observer: policyadapter.CommandObserver{}, Overrides: policyadapter.OverrideLoader{}, Executor: policyadapter.CommandExecutor{}, Clock: policyadapter.Clock{}}
	return app.Service{Store: adapter.FileStore{}, Clock: adapter.Clock{}, Runner: app.CommandRunner{Evaluate: policy.Evaluate, Execute: policy.Run}}
}

func testHandleGatesMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleGatesMCPToolCall(call, testGatesService())
}
