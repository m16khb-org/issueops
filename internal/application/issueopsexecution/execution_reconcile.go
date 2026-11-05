package issueopsexecution

import (
	issueopsport "issueops/internal/port"
)

import (
	"context"
	"fmt"

	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	reconciledomain "issueops/internal/domain/issueopsreconcile"
)

// Reconcile 메서드는 기록된 pending 외부 의도를 미리보기(req.Preview)하거나
// 확정(req.Confirm)해 재조정한다. 둘 중 정확히 하나만 지정해야 한다.
// pending이 없으면 Reconciled 결과를 바로 돌려주고, 확정 시에는 호출자의 실행 권한을 검증한 뒤 경로별 핸들러로 넘긴다.
func (s Service) Reconcile(ctx context.Context, stateRoot string, req issueops.ExecutionReconcileRequest, deps issueopsport.ExecutionReconcileDependencies) (issueops.ExecutionReconcileResult, error) {
	if req.Preview == req.Confirm {
		return issueops.ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile requires exactly one of preview or confirm (--preview, then the --confirm it returns)")
	}
	record, actor, err := s.loadReconcileTarget(ctx, stateRoot, req)
	if err != nil {
		return issueops.ExecutionReconcileResult{OK: false, ID: req.ID}, err
	}
	req.Actor = actor
	kind := ""
	if record.Execution.Pending != nil {
		kind = record.Execution.Pending.Kind
	}
	decision := reconciledomain.DecidePending(kind, record.Execution.Pending != nil)
	if req.Confirm {
		if err := s.authorizeReconcileConfirm(ctx, record, req, actor); err != nil {
			return issueops.ExecutionReconcileResult{OK: false, ID: req.ID}, err
		}
	}
	result := executionReconcileResult(record, req.Preview, "")
	if decision.Route == reconciledomain.RouteNone {
		result.Reconciled = true
		result.Code = "no_pending_external_intent"
		return result, nil
	}
	if req.Preview {
		return previewReconcileResult(result, record, decision, actor, req.CWD), nil
	}
	return dispatchReconcileRoute(ctx, stateRoot, req, deps, record, decision, result)
}

// loadReconcileTarget verifies the actor and reads the record, then checks
// that the record is prepared and req.CWD names its source root or worktree.
func (s Service) loadReconcileTarget(ctx context.Context, stateRoot string, req issueops.ExecutionReconcileRequest) (issueops.Record, issueops.NativeActor, error) {
	if s.Verifier == nil {
		return issueops.Record{}, issueops.NativeActor{}, fmt.Errorf("execution reconcile actor verifier is required")
	}
	verified, err := s.Verifier.Verify(ctx, req.Actor)
	if err != nil {
		return issueops.Record{}, issueops.NativeActor{}, err
	}
	actor := verified.Identity
	record, err := s.ReadRecord(stateRoot, req.ID)
	if err != nil {
		return issueops.Record{}, actor, err
	}
	if record.Execution == nil {
		return issueops.Record{}, actor, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	if !s.SamePath(req.CWD, record.Execution.Workspace.SourceRoot) && !s.SamePath(req.CWD, record.Execution.Workspace.Root) {
		return issueops.Record{}, actor, fmt.Errorf("execution reconcile cwd must be source_root or the canonical worktree")
	}
	return record, actor, nil
}

// authorizeReconcileConfirm는 확정 reconcile 전에 호출자가 실행 권한을 가졌는지 검증한다.
func (s Service) authorizeReconcileConfirm(ctx context.Context, record issueops.Record, req issueops.ExecutionReconcileRequest, actor issueops.NativeActor) error {
	mutationActor := issueops.Actor{
		Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: req.CWD,
		NativeProcessAncestry: actor.ProcessAncestry,
	}
	return cycleapp.NewMutationAuthority(s.SamePath, s.Verifier).Validate(ctx, record, &mutationActor)
}

// previewReconcileResult는 미리보기 결과에 판정 코드와 확정 명령을 채운다.
func previewReconcileResult(result issueops.ExecutionReconcileResult, record issueops.Record, decision reconciledomain.Decision, actor issueops.NativeActor, cwd string) issueops.ExecutionReconcileResult {
	result.Code = decision.PreviewCode
	if decision.Route == reconciledomain.RouteRemotePR {
		result.NextCommand = issueopsdomain.ReconcileConfirmCommand(record.ID, actor, cwd)
	}
	return result
}

// dispatchReconcileRoute는 결정된 경로에 맞는 reconcile 핸들러를 호출한다.
func dispatchReconcileRoute(ctx context.Context, stateRoot string, req issueops.ExecutionReconcileRequest, deps issueopsport.ExecutionReconcileDependencies, record issueops.Record, decision reconciledomain.Decision, result issueops.ExecutionReconcileResult) (issueops.ExecutionReconcileResult, error) {
	switch decision.Route {
	case reconciledomain.RouteRemotePR:
		if deps.RemoteReconcile == nil {
			return failedExecutionReconcileResult(record, "remote_reconcile_unavailable"), issueops.ErrRemotePullRequestReconcileHandlerUnavailable
		}
		req.Snapshot = &record
		return deps.RemoteReconcile(ctx, stateRoot, req)
	default:
		result.OK = false
		result.Code = "unsupported_external_intent"
		return result, fmt.Errorf("unsupported pending external intent kind %q", record.Execution.Pending.Kind)
	}
}

func executionReconcileResult(record issueops.Record, preview bool, code string) issueops.ExecutionReconcileResult {
	result := issueops.ExecutionReconcileResult{OK: true, ID: record.ID, Preview: preview, Code: code}
	if record.Execution != nil {
		result.Execution = *record.Execution
		result.Pending = record.Execution.Pending
	}
	return result
}

func failedExecutionReconcileResult(record issueops.Record, code string) issueops.ExecutionReconcileResult {
	result := executionReconcileResult(record, false, code)
	result.OK = false
	return result
}
