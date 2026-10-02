package issueopsapp

import (
	"context"

	completioninbound "issueops/internal/adapter/inbound/issueopscompletion"
	completionoutbound "issueops/internal/adapter/outbound/issueopscompletion"
	"issueops/internal/adapter/outbound/sqlstore"
	completionapp "issueops/internal/application/issueopscompletion"
	issueopscontract "issueops/internal/contract/issueops"
	completioncontract "issueops/internal/contract/issueopscompletion"
)

func issueOpsCompleteHandler(ctx context.Context, stateRoot string, request issueopscontract.ExecutionCompleteRequest) (issueopscontract.ExecutionResult, error) {
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, err
	}
	service := completionapp.NewService(
		completionoutbound.NewRepository(database), completionoutbound.NewEnvironment(), completionoutbound.UTCClock{},
		verifyIssueOpsCompletionActor,
	)
	return completioninbound.NewHandler(service)(ctx, stateRoot, request)
}

func verifyIssueOpsCompletionActor(ctx context.Context, actor completioncontract.Actor, ancestry []completioncontract.ProcessReceipt) (completioncontract.Actor, error) {
	native := issueopscontract.NativeActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		native.SessionProcess = &issueopscontract.NativeProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	for _, receipt := range ancestry {
		native.ProcessAncestry = append(native.ProcessAncestry, issueopscontract.NativeProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	verified, err := issueOpsActorVerifier().Verify(ctx, native)
	if err != nil {
		return completioncontract.Actor{}, err
	}
	identity := verified.Identity
	result := completioncontract.Actor{Host: identity.Host, SessionID: identity.SessionID, AgentID: identity.AgentID}
	if identity.SessionProcess != nil {
		result.Process = &completioncontract.ProcessReceipt{PID: identity.SessionProcess.PID, StartedAt: identity.SessionProcess.StartedAt, Executable: identity.SessionProcess.Executable}
	}
	return result, nil
}
