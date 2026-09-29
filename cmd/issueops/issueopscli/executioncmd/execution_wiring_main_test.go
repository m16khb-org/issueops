package executioncmd

import core "issueops/internal/adapter/issueops"

func testExecutionRuntime() ExecutionDeps {
	return ExecutionDeps{ExecuteExecution: core.ExecuteExecution, ObserveNativeProcessAncestry: core.ObserveNativeProcessAncestry, SwitchExecutionMode: core.SwitchExecutionMode, SyncExecutionBase: core.SyncExecutionBase}
}
func runExecutionForTest(args []string, deps Deps) error {
	defaults := testExecutionRuntime()
	if deps.Runtime.ExecuteExecution == nil {
		deps.Runtime.ExecuteExecution = defaults.ExecuteExecution
	}
	if deps.Runtime.ObserveNativeProcessAncestry == nil {
		deps.Runtime.ObserveNativeProcessAncestry = defaults.ObserveNativeProcessAncestry
	}
	if deps.Runtime.SwitchExecutionMode == nil {
		deps.Runtime.SwitchExecutionMode = defaults.SwitchExecutionMode
	}
	if deps.Runtime.SyncExecutionBase == nil {
		deps.Runtime.SyncExecutionBase = defaults.SyncExecutionBase
	}
	return Run(args, deps)
}
