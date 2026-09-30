package mcpcli

import (
	adapter "issueops/internal/adapter/gates"
	policyadapter "issueops/internal/adapter/policy"
	app "issueops/internal/application/gates"
)

func testGatesService() app.Service {
	return app.Service{Store: adapter.FileStore{}, Clock: adapter.Clock{}, Runner: app.CommandRunner{Evaluate: policyadapter.EvaluateCommandPolicy, Execute: policyadapter.RunCommand}}
}

func testHandleGatesMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleGatesMCPToolCall(call, testGatesService())
}
