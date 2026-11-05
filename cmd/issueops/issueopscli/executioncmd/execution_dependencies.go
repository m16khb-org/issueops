package executioncmd

import (
	"context"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// ExecutionDeps 타입은 execution 명령이 쓰는 런타임 함수 묶음이다. ExecuteExecution은 실행 액션을
// 수행하고, ObserveNativeProcessAncestry는 PID의 프로세스 조상 영수증을 관측하며,
// SyncExecutionBase는 base 동기화를 수행한다.
type ExecutionDeps struct {
	ExecuteExecution             func(context.Context, string, issueopscontract.ExecutionActionRequest, port.ExecutionActionDependencies) (any, error)
	ObserveNativeProcessAncestry func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	SyncExecutionBase            func(context.Context, string, issueopscontract.ExecutionSyncBaseRequest, issueopscontract.ExecutionSyncBaseDeps) (issueopscontract.ExecutionSyncBaseResult, error)
}
