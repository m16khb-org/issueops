package issueopsexecution

import (
	"context"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func invokeExecutionPrepareHandler(ctx context.Context, stateRoot string, request model.ExecutionPrepareRequest, invocation executionissue.ExecutionPrepareInvocation, handler model.ExecutionPrepareHandler) (model.ExecutionPrepareResult, error) {
	if handler == nil {
		return model.ExecutionPrepareResult{ID: request.ID}, model.ErrPrepareHandlerUnavailable
	}
	return handler(ctx, stateRoot, request, invocation)
}

func (s Service) Execute(ctx context.Context, stateRoot string, req model.ExecutionActionRequest, deps port.ExecutionActionDependencies) (any, error) {
	readIssue, snapshotSource, err := s.snapshotReader(stateRoot, req, deps.ReadIssue)
	if err != nil {
		return nil, err
	}
	deps.ReadIssue = readIssue
	result, err := s.executeAction(ctx, stateRoot, req, deps)
	if err != nil {
		return result, err
	}
	return withExecutionIssueSnapshotSource(result, snapshotSource()), nil
}

func (s Service) executeAction(ctx context.Context, stateRoot string, req model.ExecutionActionRequest, deps port.ExecutionActionDependencies) (any, error) {
	switch req.Action {
	case model.ExecutionActionPrepare:
		return invokeExecutionPrepareHandler(ctx, stateRoot, model.ExecutionPrepareRequest{
			ID: req.ID, Mode: req.Mode, Actor: req.Actor, CWD: req.CWD,
			OwnerHost: req.OwnerHost, OwnerModel: req.OwnerModel, OwnerEffort: req.OwnerEffort,
			IssueSnapshotFile: req.IssueSnapshotFile,
			DirectReason:      req.DirectReason, ExpectedReadinessFingerprint: req.ExpectedReadinessFingerprint, Confirm: req.Confirm,
		}, executionissue.ExecutionPrepareInvocation{ReadIssue: deps.ReadIssue}, deps.Prepare)
	case model.ExecutionActionStatus:
		if deps.Status == nil {
			return model.ExecutionResult{OK: false, ID: req.ID}, fmt.Errorf("issueops execution status handler is not configured")
		}
		return deps.Status(ctx, stateRoot, req.ID)
	case model.ExecutionActionClaim:
		if deps.Claim == nil {
			return model.ExecutionResult{OK: false, ID: req.ID}, model.ErrClaimHandlerUnavailable
		}
		return deps.Claim(ctx, stateRoot, model.ExecutionClaimRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD, TokenFile: req.TokenFile, ClaimCurrentToken: req.ClaimCurrentToken,
			IssueBodySHA256: req.IssueBodySHA256, ContextPacketSHA256: req.ContextPacketSHA256,
		}, model.ExecutionClaimDependencies{ReadIssue: deps.ReadIssue})
	case model.ExecutionActionRelease:
		if deps.Release == nil {
			return model.ExecutionResult{OK: false, ID: req.ID}, model.ErrReleaseHandlerUnavailable
		}
		return deps.Release(ctx, stateRoot, model.ExecutionReleaseRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
		})
	case model.ExecutionActionReplace:
		if req.ReplaceAction == model.ExecutionReplaceReseed {
			if deps.Reseed == nil {
				return model.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.ReplaceAction}, model.ErrReseedHandlerUnavailable
			}
			return deps.Reseed(ctx, stateRoot, model.ExecutionReseedRequest{
				ID: req.ID, ExpectedGeneration: req.ExpectedGeneration, CompletionGeneration: req.CompletionGeneration, InventoryFingerprint: req.InventoryFingerprint,
				Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
				ReadIssue: deps.ReadIssue,
			})
		}
		if deps.Replace == nil {
			return model.ExecutionReplaceResult{ID: req.ID, Action: req.ReplaceAction}, fmt.Errorf("issueops execution replace handler is not configured")
		}
		return deps.Replace(ctx, stateRoot, model.ExecutionReplaceRequest{
			ID: req.ID, Action: req.ReplaceAction, ExpectedGeneration: req.ExpectedGeneration, CompletionGeneration: req.CompletionGeneration,
			InventoryFingerprint: req.InventoryFingerprint, QuiescenceFingerprint: req.QuiescenceFingerprint,
			Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm, ReadIssue: deps.ReadIssue,
			// finalize/reseed 재봉인이 현재 이슈 본문을 다시 읽어야 하므로
			// prepare/claim과 같은 리더를 함께 넘긴다.
		}, port.ReplacementInvocation{OrcaOwner: deps.OrcaOwner, BaseSync: deps.BaseSync})
	case model.ExecutionActionResume:
		if !req.Confirm {
			return model.ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires confirm")
		}
		if deps.Resume == nil {
			return model.ExecutionResumeResult{OK: false, ID: req.ID}, model.ErrResumeHandlerUnavailable
		}
		return deps.Resume(ctx, stateRoot, model.ExecutionResumeRequest{
			ID: req.ID, ExpectedGeneration: req.ExpectedGeneration,
			Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
		})
	case model.ExecutionActionReconcile:
		return s.Reconcile(ctx, stateRoot, model.ExecutionReconcileRequest{
			ID: req.ID, Preview: req.Preview, Confirm: req.Confirm, Actor: req.Actor, CWD: req.CWD,
		}, port.ExecutionReconcileDependencies{
			Orca: deps.Orca, ReadIssue: deps.ReadIssue,
			Handler: deps.Reconcile, RemoteReconcile: deps.RemoteReconcile,
		})
	case model.ExecutionActionComplete:
		if deps.Complete == nil {
			return model.ExecutionResult{OK: false, ID: req.ID}, model.ErrCompleteHandlerUnavailable
		}
		return deps.Complete(ctx, stateRoot, model.ExecutionCompleteRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
			FinalHead: req.FinalHead, VerificationReportPath: req.VerificationReportPath,
			Verification: req.Verification, RemoteArtifactURL: req.RemoteArtifactURL, Confirm: req.Confirm,
		})
	default:
		return nil, fmt.Errorf("unsupported issueops execution action %q", req.Action)
	}
}
