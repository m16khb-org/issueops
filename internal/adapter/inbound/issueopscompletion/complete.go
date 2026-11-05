// Package issueopscompletion maps execution requests to the completion
// application service.
package issueopscompletion

import (
	"context"
	"errors"

	completionapp "issueops/internal/application/issueopscompletion"
	issueopscontract "issueops/internal/contract/issueops"
	completioncontract "issueops/internal/contract/issueopscompletion"
	statecontract "issueops/internal/contract/state"
)

type service interface {
	Complete(context.Context, completionapp.Request) (completionapp.Result, error)
}

type handler struct{ service service }

// NewHandler returns the execution-complete handler backed by service. The
// handler reports ErrCompleteHandlerUnavailable when service is nil.
func NewHandler(service service) issueopscontract.ExecutionCompleteHandler {
	return handler{service: service}.Handle
}

func (h handler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionCompleteRequest) (issueopscontract.ExecutionResult, error) {
	if h.service == nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, issueopscontract.ErrCompleteHandlerUnavailable
	}
	result, err := h.service.Complete(ctx, completionapp.Request{
		ID: request.ID, Generation: request.Generation, Actor: completionActor(request.Actor), Ancestry: completionAncestry(request.Actor),
		CWD: request.CWD, FinalHead: request.FinalHead, VerificationReportPath: request.VerificationReportPath,
		Verification: append([]string(nil), request.Verification...), RemoteArtifactURL: request.RemoteArtifactURL, Confirm: request.Confirm,
	})
	if err != nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, publicError(err)
	}
	return issueopscontract.ExecutionResult{OK: result.OK, ID: result.ID, Execution: result.Execution.ToIssueOps()}, nil
}

func completionActor(actor issueopscontract.NativeActor) completioncontract.Actor {
	result := completioncontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.Process = &completioncontract.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}
	}
	return result
}

func completionAncestry(actor issueopscontract.NativeActor) []completioncontract.ProcessReceipt {
	result := make([]completioncontract.ProcessReceipt, 0, len(actor.ProcessAncestry))
	for _, receipt := range actor.ProcessAncestry {
		result = append(result, completioncontract.ProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	return result
}

func publicError(err error) error {
	if errors.Is(err, statecontract.ErrInvalidState) {
		return statecontract.ErrInvalidState
	}
	return err
}
