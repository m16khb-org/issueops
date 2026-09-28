package issueops

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"issueops/internal/contract/issueops"
	reconciledomain "issueops/internal/domain/issueopsreconcile"
	"issueops/internal/port"
)

func ReconcileExecutionWithDependencies(ctx context.Context, stateRoot string, req ExecutionReconcileRequest, deps ExecutionReconcileDependencies) (ExecutionReconcileResult, error) {
	if req.Preview == req.Confirm {
		return ExecutionReconcileResult{OK: false, ID: req.ID}, fmt.Errorf("execution reconcile requires exactly one of preview or confirm")
	}
	actor, err := normalizeNativeActor(req.Actor)
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

func markRemotePullRequestRetry(stateRoot, id string, expected externalRemotePRPayload) (externalRemotePRPayload, error) {
	updated := expected
	updated.InvocationState = remoteInvocationUnknown
	updated.RetryCount++
	data, err := jsonMarshalExecutionIntent(updated)
	if err != nil {
		return externalRemotePRPayload{}, err
	}
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != expected.OperationID {
			return fmt.Errorf("external intent changed before retry CAS")
		}
		current, err := readExternalRemotePRPayload(stateRoot, expected.OperationID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, expected) {
			return fmt.Errorf("external intent payload changed before retry CAS")
		}
		_, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []port.RecordMutation{{Bucket: externalIntentBucket, ID: expected.OperationID, Data: data}})
		return err
	})
	return updated, err
}

func finishRemotePullRequestPreInvocationFailure(stateRoot, id string, payload externalRemotePRPayload, cause error, now func() time.Time) (issueops.IssueOpsRecord, error) {
	var persisted issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != payload.OperationID {
			return fmt.Errorf("external intent changed before terminal pre-invocation receipt")
		}
		current, err := readExternalRemotePRPayload(stateRoot, payload.OperationID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, payload) {
			return fmt.Errorf("external intent payload changed before terminal pre-invocation receipt")
		}
		record.Execution.Pending = nil
		record.Execution.Failure = &issueops.ExecutionFailure{
			OperationID: payload.OperationID, Code: "external_operation_not_invoked", Message: boundedExecutionRemoteDiagnostic(cause), At: executionNow(now),
		}
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []port.RecordMutation{{Bucket: externalIntentBucket, ID: payload.OperationID, Delete: true}})
		return err
	})
	return persisted, err
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

func jsonMarshalExecutionIntent(payload externalRemotePRPayload) ([]byte, error) {
	return json.Marshal(payload)
}
