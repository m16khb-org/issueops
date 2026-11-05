// Package feedbackcleanup 패키지는 issueops feedback 명령과 cleanup 명령을 해석하고
// 실행한다. 플래그를 읽고 주입된 연산을 호출해 결과를 출력한다. cmd 계층이다.
package feedbackcleanup

import (
	"context"

	cleanupapp "issueops/internal/application/issueopscleanup"
	issueopscontract "issueops/internal/contract/issueops"
)

// Command 타입은 feedback·cleanup 명령의 실행 단위다. Operations가 상태 저장소
// 연산을, Invoke가 cleanup 유스케이스 호출 조립을 맡는다.
type Command struct {
	Operations CleanupDeps
	Invoke     func(Deps) cleanupapp.Invocation
}

// CleanupDeps 타입은 feedback·cleanup 명령이 쓰는 상태 저장소 연산 묶음이다.
type CleanupDeps struct {
	Status                                            func(context.Context, string, string, bool, Deps) (issueopscontract.CleanupStatus, error)
	AddIssueOpsFeedbackWithActor                      func(stateRoot, id, source, body, classification string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	CloseIssueOpsChildren                             func(stateRoot, id string, req issueopscontract.CloseChildrenRequest, deps Deps) (issueopscontract.CloseChildrenResult, error)
	IssueOpsStateRoot                                 func() string
	MarkIssueOpsContractFeedbackIssueUpdatedWithActor func(stateRoot, id string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	ObserveNativeProcessAncestry                      func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
}
