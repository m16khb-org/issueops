// Package issueopspreparation maps execution requests to the preparation
// application service.
package issueopspreparation

import (
	"context"
	"errors"

	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	statecontract "issueops/internal/contract/state"
)

type service interface {
	Prepare(context.Context, preparationcontract.Command) (preparationcontract.Result, error)
}

type Handler struct{ service service }

// 반환 타입은 어댑터의 이름 붙은 핸들러 타입 대신 같은 시그니처를 직접 쓴다.
// Go에서 두 형태는 할당 호환이므로 소비자는 그대로 동작하고, inbound 어댑터는
// issueops 어댑터를 알 필요가 없어진다.
func NewHandler(service service) func(context.Context, string, issueopscontract.ExecutionPrepareRequest) (issueopscontract.ExecutionPrepareResult, error) {
	return Handler{service: service}.Handle
}

func (handler Handler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionPrepareRequest) (issueopscontract.ExecutionPrepareResult, error) {
	if handler.service == nil {
		return issueopscontract.ExecutionPrepareResult{ID: request.ID}, issueopscontract.ErrPrepareHandlerUnavailable
	}
	result, err := handler.service.Prepare(ctx, preparationcontract.Command{
		ID: request.ID, Actor: preparationActor(request.Actor), CWD: request.CWD, Confirm: request.Confirm,
	})
	return coreResult(result), publicError(err)
}

func preparationActor(actor issueopscontract.NativeActor) leasecontract.Actor {
	result := leasecontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	return result
}

func coreResult(result preparationcontract.Result) issueopscontract.ExecutionPrepareResult {
	response := issueopscontract.ExecutionPrepareResult{
		OK: result.OK, ID: result.ID, Preview: result.Preview,
		Workspace:      result.Workspace.ToIssueOps(),
		ClaimTokenPath: result.ClaimTokenPath, NextCommand: result.NextCommand,
	}
	if result.Execution != nil {
		execution := result.Execution.ToIssueOps()
		response.Execution = &execution
	}
	return response
}

func publicError(err error) error {
	if errors.Is(err, statecontract.ErrInvalidState) {
		return statecontract.ErrInvalidState
	}
	return err
}
