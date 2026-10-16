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
	return issueopscontract.ExecutionPrepareResult{
		OK: result.OK, ID: result.ID, Preview: result.Preview,
		Workspace: coreWorkspace(result.Workspace), Execution: coreExecution(result.Execution),
		ClaimTokenPath: result.ClaimTokenPath, NextCommand: result.NextCommand,
	}
}

func coreWorkspace(workspace leasecontract.Workspace) issueopscontract.Workspace {
	return issueopscontract.Workspace{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
		BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree, LinkedAt: workspace.LinkedAt,
	}
}

func coreExecution(execution *leasecontract.Execution) *issueopscontract.Execution {
	if execution == nil {
		return nil
	}
	result := &issueopscontract.Execution{
		Workspace: coreWorkspace(execution.Workspace),
		Lease: issueopscontract.WriteLease{
			Generation: execution.Lease.Generation, Status: issueopscontract.LeaseStatus(execution.Lease.Status),
			ClaimTokenSHA256: execution.Lease.ClaimTokenSHA256, ClaimedAt: execution.Lease.ClaimedAt,
			ReleasedAt: execution.Lease.ReleasedAt, ReplacedAt: execution.Lease.ReplacedAt, ReplacementReason: execution.Lease.ReplacementReason,
		},
	}
	if execution.Lease.Holder != nil {
		holder := execution.Lease.Holder
		result.Lease.Holder = &issueopscontract.NativeActor{Host: holder.Host, SessionID: holder.SessionID, AgentID: holder.AgentID}
		if holder.SessionProcess != nil {
			result.Lease.Holder.SessionProcess = &issueopscontract.NativeProcessReceipt{
				PID: holder.SessionProcess.PID, StartedAt: holder.SessionProcess.StartedAt, Executable: holder.SessionProcess.Executable,
			}
		}
	}
	if execution.Pending != nil {
		pending := execution.Pending
		result.Pending = &issueopscontract.ExternalIntent{OperationID: pending.OperationID, Kind: pending.Kind, Marker: pending.Marker, StartedAt: pending.StartedAt}
	}
	if execution.Completion != nil {
		completion := execution.Completion
		result.Completion = &issueopscontract.ExecutionCompletion{
			Generation: completion.Generation, FinalHead: completion.FinalHead, VerificationReportPath: completion.VerificationReportPath,
			Verification: cloneStrings(completion.Verification), RemoteArtifactURL: completion.RemoteArtifactURL, CompletedAt: completion.CompletedAt,
		}
	}
	if execution.CompletionHistory != nil {
		result.CompletionHistory = make([]issueopscontract.ExecutionCompletionHistory, len(execution.CompletionHistory))
		for index, entry := range execution.CompletionHistory {
			result.CompletionHistory[index] = issueopscontract.ExecutionCompletionHistory{
				Generation: entry.Generation,
				Completion: issueopscontract.ExecutionCompletion{Generation: entry.Completion.Generation, FinalHead: entry.Completion.FinalHead, VerificationReportPath: entry.Completion.VerificationReportPath, Verification: cloneStrings(entry.Completion.Verification), RemoteArtifactURL: entry.Completion.RemoteArtifactURL, CompletedAt: entry.Completion.CompletedAt},
				Reason:     entry.Reason,
				ReopenedAt: entry.ReopenedAt,
			}
		}
	}
	if execution.Failure != nil {
		failure := execution.Failure
		result.Failure = &issueopscontract.ExecutionFailure{OperationID: failure.OperationID, Code: failure.Code, Message: failure.Message, At: failure.At}
	}
	if execution.SyncBaseEvents != nil {
		result.SyncBaseEvents = make([]issueopscontract.ExecutionSyncBaseEvent, len(execution.SyncBaseEvents))
		for index, event := range execution.SyncBaseEvents {
			result.SyncBaseEvents[index] = issueopscontract.ExecutionSyncBaseEvent{
				Mode: event.Mode, BaseBranch: event.BaseBranch, BaseOID: event.BaseOID, MergeCommit: event.MergeCommit,
				ConflictFiles: event.ConflictFiles, Actor: event.Actor, At: event.At,
			}
		}
	}
	return result
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func publicError(err error) error {
	if errors.Is(err, statecontract.ErrInvalidState) {
		return statecontract.ErrInvalidState
	}
	return err
}
