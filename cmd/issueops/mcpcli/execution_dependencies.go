package mcpcli

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type ExecutionDeps struct {
	ExecuteExecution             func(context.Context, string, issueopscontract.ExecutionActionRequest, port.ExecutionActionDependencies) (any, error)
	ObserveNativeProcessAncestry func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	IssueOpsStateRoot            func() string
}
