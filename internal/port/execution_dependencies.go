package port

import (
	"context"

	issueopscontract "issueops/internal/contract/issueops"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

// ExecutionStatusHandler 타입은 실행 상태 조회 함수 타입이다. 실행 의존 묶음은 port 인터페이스를 필드로 갖는다. DTO가 아니라 주입 묶음이므로 계약이
// 아니라 port 계층이 소유한다 — 계약은 인터페이스를 물지 않는다.
type ExecutionStatusHandler func(context.Context, string, string) (issueopscontract.ExecutionResult, error)

// ExecutionActionDependencies bundles the per-action handlers that the execution service dispatches
// to (status, prepare, base sync, claim, release, replace, reseed, complete and remote PR
// reconcile). Callers inject only what the requested action needs.
type ExecutionActionDependencies struct {
	Status   ExecutionStatusHandler
	Prepare  issueopscontract.ExecutionPrepareHandler
	BaseSync basesyncport.Inspector
	Claim    issueopscontract.ExecutionClaimHandler
	Release  issueopscontract.ExecutionReleaseHandler
	Replace  ExecutionReplaceHandler
	Reseed   issueopscontract.ExecutionReseedHandler
	Complete issueopscontract.ExecutionCompleteHandler
	// RemoteReconcile handles remote_pr_create recovery.
	RemoteReconcile issueopscontract.RemotePullRequestReconcileHandler
}

// ExecutionReconcileDependencies carries the handler used to reconcile a remote pull request create
// that may have been interrupted.
type ExecutionReconcileDependencies struct {
	RemoteReconcile issueopscontract.RemotePullRequestReconcileHandler
}
