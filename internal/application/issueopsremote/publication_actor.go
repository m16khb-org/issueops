package issueopsremote

import (
	"context"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	authorityport "issueops/internal/port/authority"
)

func publicationNativeActor(actor contract.Actor) model.NativeActor {
	result := model.NativeActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &model.NativeProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	if actor.ProcessAncestry != nil {
		result.ProcessAncestry = make([]model.NativeProcessReceipt, len(actor.ProcessAncestry))
		for index, receipt := range actor.ProcessAncestry {
			result.ProcessAncestry[index] = model.NativeProcessReceipt{
				PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable,
			}
		}
	}
	return result
}

func normalizePublicationActor(ctx context.Context, actor contract.Actor, verifier authorityport.ActorVerifier) (contract.Actor, error) {
	verified, err := verifier.Verify(ctx, publicationNativeActor(actor))
	if err != nil {
		return contract.Actor{}, err
	}
	normalized := verified.Identity
	result := actor.Clone()
	result.Host, result.SessionID, result.AgentID = normalized.Host, normalized.SessionID, normalized.AgentID
	if normalized.SessionProcess != nil {
		receipt := contract.ProcessReceipt(*normalized.SessionProcess)
		result.SessionProcess = &receipt
	}
	return result, nil
}

func publicationMutationActor(command contract.CreateCommand) model.IssueOpsActor {
	actor := publicationNativeActor(command.Actor)
	return model.IssueOpsActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: command.CWD, NativeProcessAncestry: actor.ProcessAncestry}
}
