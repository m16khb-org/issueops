package issueops

import (
	"context"
	"fmt"

	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	reconciledomain "issueops/internal/domain/issueopsreconcile"
)

func ReconcileExecutionWithDependencies(ctx context.Context, stateRoot string, req ExecutionReconcileRequest, deps ExecutionReconcileDependencies) (ExecutionReconcileResult, error) {
	if req.Preview == req.Confirm {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile requires exactly one of preview or confirm")
	}
	actor, err := cycleapp.NormalizeNativeActor(req.Actor, inspectNativeProcessReceipt)
	if err != nil {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, err
	}
	req.Actor = actor
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, err
	}
	if record.Execution == nil {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	if !samePath(req.CWD, record.Execution.Workspace.SourceRoot) && !samePath(req.CWD, record.Execution.Workspace.Root) {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile cwd must be source_root or the canonical worktree")
	}
	kind := ""
	if record.Execution.Pending != nil {
		kind = record.Execution.Pending.Kind
	}
	decision := reconciledomain.DecidePending(string(record.Execution.Mode), kind, record.Execution.Pending != nil)
	if req.Confirm && !decision.SkipMutationGuard {
		mutationActor := IssueOpsActor{
			Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: req.CWD,
			NativeProcessAncestry: actor.ProcessAncestry,
		}
		if err := validateExecutionMutation(record, &mutationActor); err != nil {
			return ExecutionReconcileResult{OK: false, ID: req.ID}, err
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
			return failedExecutionReconcileResult(record, "remote_reconcile_unavailable"), ErrRemotePullRequestReconcileHandlerUnavailable
		}
		req.Snapshot = &record
		return deps.RemoteReconcile(ctx, stateRoot, req)
	case reconciledomain.RouteOrca:
		if deps.Handler == nil {
			return failedExecutionReconcileResult(record, "orca_reconcile_ambiguous"), ErrReconcileHandlerUnavailable
		}
		req.Snapshot = &record
		return deps.Handler(ctx, stateRoot, req, deps)
	default:
		result.OK = false
		result.Code = "unsupported_external_intent"
		return result, fmt.Errorf("unsupported pending external intent kind %q", record.Execution.Pending.Kind)
	}
}

func executionReconcileResult(record issueops.IssueOpsRecord, preview bool, code string) ExecutionReconcileResult {
	result := ExecutionReconcileResult{OK: true, ID: record.ID, Preview: preview, Code: code}
	if record.Execution != nil {
		result.Execution = *record.Execution
		result.Pending = record.Execution.Pending
	}
	return result
}

func failedExecutionReconcileResult(record issueops.IssueOpsRecord, code string) ExecutionReconcileResult {
	result := executionReconcileResult(record, false, code)
	result.OK = false
	return result
}
