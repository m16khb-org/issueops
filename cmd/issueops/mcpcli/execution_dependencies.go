package mcpcli

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// ExecutionDeps 타입은 issueops_execution 도구가 호출하는 실행 서비스, 프로세스 계보 관측기, 상태 루트 조회를 묶는다.
type ExecutionDeps struct {
	ExecuteExecution             func(context.Context, string, issueopscontract.ExecutionActionRequest, port.ExecutionActionDependencies) (any, error)
	ObserveNativeProcessAncestry func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	IssueOpsStateRoot            func() string
}
