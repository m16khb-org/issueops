package issueopsexecution

import (
	issueopsport "issueops/internal/port"
)

import (
	"context"
	"fmt"

	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	reconciledomain "issueops/internal/domain/issueopsreconcile"
)

func (s Service) Reconcile(ctx context.Context, stateRoot string, req issueops.ExecutionReconcileRequest, deps issueopsport.ExecutionReconcileDependencies) (issueops.ExecutionReconcileResult, error) {
	if req.Preview == req.Confirm {
		return issueops.ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile requires exactly one of preview or confirm")
	}
	actor, err := cycleapp.NormalizeNativeActor(req.Actor, s.InspectProcess)
	if err != nil {
		return issueops.ExecutionReconcileResult{OK: false, ID: req.ID}, err
	}
	req.Actor = actor
	record, err := s.ReadRecord(stateRoot, req.ID)
	if err != nil {
		return issueops.ExecutionReconcileResult{OK: false, ID: req.ID}, err
	}
	if record.Execution == nil {
		return issueops.ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	if !s.SamePath(req.CWD, record.Execution.Workspace.SourceRoot) && !s.SamePath(req.CWD, record.Execution.Workspace.Root) {
		return issueops.ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile cwd must be source_root or the canonical worktree")
	}
	kind := ""
	if record.Execution.Pending != nil {
		kind = record.Execution.Pending.Kind
	}
	decision := reconciledomain.DecidePending(string(record.Execution.Mode), kind, record.Execution.Pending != nil)
	if req.Confirm && !decision.SkipMutationGuard {
		mutationActor := issueops.IssueOpsActor{
			Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: req.CWD,
			NativeProcessAncestry: actor.ProcessAncestry,
		}
		if err := cycleapp.NewMutationAuthority(s.SamePath).Validate(record, &mutationActor); err != nil {
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
		result.Code = decision.PreviewCode
		return result, nil
	}
	switch decision.Route {
	case reconciledomain.RouteRemotePR:
		if deps.RemoteReconcile == nil {
			return failedExecutionReconcileResult(record, "remote_reconcile_unavailable"), issueops.ErrRemotePullRequestReconcileHandlerUnavailable
		}
		req.Snapshot = &record
		return deps.RemoteReconcile(ctx, stateRoot, req)
	case reconciledomain.RouteOrca:
		if deps.Handler == nil {
			return failedExecutionReconcileResult(record, "orca_reconcile_ambiguous"), issueops.ErrReconcileHandlerUnavailable
		}
		req.Snapshot = &record
		return deps.Handler(ctx, stateRoot, req, deps)
	default:
		result.OK = false
		result.Code = "unsupported_external_intent"
		return result, fmt.Errorf("unsupported pending external intent kind %q", record.Execution.Pending.Kind)
	}
}

func executionReconcileResult(record issueops.IssueOpsRecord, preview bool, code string) issueops.ExecutionReconcileResult {
	result := issueops.ExecutionReconcileResult{OK: true, ID: record.ID, Preview: preview, Code: code}
	if record.Execution != nil {
		result.Execution = *record.Execution
		result.Pending = record.Execution.Pending
	}
	return result
}

func failedExecutionReconcileResult(record issueops.IssueOpsRecord, code string) issueops.ExecutionReconcileResult {
	result := executionReconcileResult(record, false, code)
	result.OK = false
	return result
}
