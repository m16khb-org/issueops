package executioncmd

import (
	"context"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type ExecutionDeps struct {
	ExecuteExecution             func(context.Context, string, issueopscontract.ExecutionActionRequest, port.ExecutionActionDependencies) (any, error)
	ObserveNativeProcessAncestry func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	SwitchExecutionMode          func(context.Context, string, issueopscontract.ExecutionSwitchModeRequest, issueopscontract.ExecutionSwitchModeDependencies) (issueopscontract.ExecutionSwitchModeResult, error)
	SyncExecutionBase            func(context.Context, string, issueopscontract.ExecutionSyncBaseRequest, issueopscontract.ExecutionSyncBaseDeps) (issueopscontract.ExecutionSyncBaseResult, error)
}
