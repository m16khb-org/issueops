// Package issueopsexecution 패키지는 IssueOps execution 수명주기의 action 분기와
// pending 외부 의도 reconcile을 담당하는 애플리케이션 서비스다.
package issueopsexecution

import (
	"context"
	"fmt"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// Execute 메서드는 req.Action에 맞는 execution 핸들러(prepare, status, claim,
// release, replace, reconcile, complete)를 deps에서 골라 호출하고 그 결과를 반환한다.
// 해당 핸들러가 없으면 사용할 수 없다는 오류를, 알 수 없는 action이면 지원하지 않는다는 오류를 돌려준다.
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
		return executeClaim(ctx, stateRoot, req, deps)
	case model.ExecutionActionRelease:
		return executeRelease(ctx, stateRoot, req, deps)
	case model.ExecutionActionReplace:
		return executeReplace(ctx, stateRoot, req, deps)
	case model.ExecutionActionReconcile:
		return s.Reconcile(ctx, stateRoot, model.ExecutionReconcileRequest{
			ID: req.ID, Preview: req.Preview, Confirm: req.Confirm, Actor: req.Actor, CWD: req.CWD,
		}, port.ExecutionReconcileDependencies{RemoteReconcile: deps.RemoteReconcile})
	case model.ExecutionActionComplete:
		return executeComplete(ctx, stateRoot, req, deps)
	default:
		// Probe: an MCP call with action "bogus" named no valid action.
		return nil, fmt.Errorf("unsupported issueops execution action %q (want prepare|status|claim|release|replace|reconcile|complete)", req.Action)
	}
}

func executeClaim(ctx context.Context, stateRoot string, req model.ExecutionActionRequest, deps port.ExecutionActionDependencies) (any, error) {
	if deps.Claim == nil {
		return model.ExecutionResult{OK: false, ID: req.ID}, model.ErrClaimHandlerUnavailable
	}
	return deps.Claim(ctx, stateRoot, model.ExecutionClaimRequest{
		ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD, TokenFile: req.TokenFile, ClaimCurrentToken: req.ClaimCurrentToken,
	})
}

func executeRelease(ctx context.Context, stateRoot string, req model.ExecutionActionRequest, deps port.ExecutionActionDependencies) (any, error) {
	if deps.Release == nil {
		return model.ExecutionResult{OK: false, ID: req.ID}, model.ErrReleaseHandlerUnavailable
	}
	return deps.Release(ctx, stateRoot, model.ExecutionReleaseRequest{
		ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
	})
}

func executeComplete(ctx context.Context, stateRoot string, req model.ExecutionActionRequest, deps port.ExecutionActionDependencies) (any, error) {
	if deps.Complete == nil {
		return model.ExecutionResult{OK: false, ID: req.ID}, model.ErrCompleteHandlerUnavailable
	}
	return deps.Complete(ctx, stateRoot, model.ExecutionCompleteRequest{
		ID: req.ID, Generation: req.Generation, Actor: req.Actor, CWD: req.CWD,
		FinalHead: req.FinalHead, VerificationReportPath: req.VerificationReportPath,
		Verification: req.Verification, RemoteArtifactURL: req.RemoteArtifactURL, Confirm: req.Confirm,
	})
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
