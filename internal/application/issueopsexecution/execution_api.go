package issueopsexecution

import (
	"context"
	"fmt"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func (s Service) Execute(ctx context.Context, stateRoot string, req model.ExecutionActionRequest, deps port.ExecutionActionDependencies) (any, error) {
	switch req.Action {
	case model.ExecutionActionPrepare:
		if deps.Prepare == nil {
			return model.ExecutionPrepareResult{ID: req.ID}, model.ErrPrepareHandlerUnavailable
		}
		return deps.Prepare(ctx, stateRoot, model.ExecutionPrepareRequest{ID: req.ID, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm})
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
		})
	case model.ExecutionActionRelease:
		if deps.Release == nil {
			return model.ExecutionResult{OK: false, ID: req.ID}, model.ErrReleaseHandlerUnavailable
		}
		return deps.Release(ctx, stateRoot, model.ExecutionReleaseRequest{
			ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
		})
	case model.ExecutionActionReplace:
		return executeReplace(ctx, stateRoot, req, deps)
	case model.ExecutionActionReconcile:
		return s.Reconcile(ctx, stateRoot, model.ExecutionReconcileRequest{
			ID: req.ID, Preview: req.Preview, Confirm: req.Confirm, Actor: req.Actor, CWD: req.CWD,
		}, port.ExecutionReconcileDependencies{RemoteReconcile: deps.RemoteReconcile})
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
		// Probe: an MCP call with action "bogus" named no valid action.
		return nil, fmt.Errorf("unsupported issueops execution action %q (want prepare|status|claim|release|replace|reconcile|complete)", req.Action)
	}
}

func executeReplace(ctx context.Context, stateRoot string, req model.ExecutionActionRequest, deps port.ExecutionActionDependencies) (any, error) {
	if req.ReplaceAction == model.ExecutionReplaceReseed {
		if deps.Reseed == nil {
			return model.ExecutionReplaceResult{OK: false, ID: req.ID, Action: req.ReplaceAction}, model.ErrReseedHandlerUnavailable
		}
		return deps.Reseed(ctx, stateRoot, model.ExecutionReseedRequest{
			ID: req.ID, ExpectedGeneration: req.ExpectedGeneration, CompletionGeneration: req.CompletionGeneration, InventoryFingerprint: req.InventoryFingerprint,
			Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
		})
	}
	if deps.Replace == nil {
		return model.ExecutionReplaceResult{ID: req.ID, Action: req.ReplaceAction}, fmt.Errorf("issueops execution replace handler is not configured")
	}
	return deps.Replace(ctx, stateRoot, model.ExecutionReplaceRequest{
		ID: req.ID, Action: req.ReplaceAction, ExpectedGeneration: req.ExpectedGeneration, CompletionGeneration: req.CompletionGeneration,
		InventoryFingerprint: req.InventoryFingerprint, QuiescenceFingerprint: req.QuiescenceFingerprint,
		Reason: req.Reason, Actor: req.Actor, CWD: req.CWD, Confirm: req.Confirm,
	}, port.ReplacementInvocation{BaseSync: deps.BaseSync})
}
