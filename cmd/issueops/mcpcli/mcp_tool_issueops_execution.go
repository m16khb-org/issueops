package mcpcli

import (
	"context"
	"os"

	"issueops/cmd/issueops/mcpcli/argmap"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func handleMCPIssueOpsExecutionWithContext(
	ctx context.Context,
	args map[string]any,
	deps MCPDependencies,
) MCPToolOutcome {
	req, err := executionActionRequestFromMCP(args, deps)
	if err != nil {
		return mcpToolErrorPayload(issueOpsMCPErrorPayload(err))
	}
	result, err := deps.Execution.ExecuteExecution(ctx, deps.Execution.IssueOpsStateRoot(), req, issueOpsExecutionActionDependencies(deps))
	if err != nil {
		err = bindMCPIssueOpsExecutionErrorNextCommand(err, deps.Provenance)
		return mcpToolErrorPayload(issueOpsMCPErrorPayload(err))
	}
	result, err = bindMCPIssueOpsExecutionNextCommand(result, deps.Provenance)
	if err != nil {
		return mcpToolErrorPayload(issueOpsMCPErrorPayload(err))
	}
	return mcpToolPayload(result)
}

func issueOpsExecutionActionDependencies(deps MCPDependencies) port.ExecutionActionDependencies {
	return port.ExecutionActionDependencies{
		Prepare: deps.Prepare, BaseSync: deps.BaseSync,
		Claim: deps.Claim, Release: deps.Release, Reseed: deps.Reseed, Status: deps.Status, Replace: deps.Replace, Complete: deps.Complete,
		RemoteReconcile: deps.Publication.Reconcile,
	}
}

func issueOpsMCPErrorPayload(err error) map[string]any {
	payload := map[string]any{"ok": false, "error": err.Error()}
	if structured, ok := err.(interface{ IssueOpsErrorFields() map[string]any }); ok {
		for key, value := range structured.IssueOpsErrorFields() {
			if value != nil && value != "" {
				payload[key] = value
			}
		}
	}
	return payload
}

func executionActionRequestFromMCP(args map[string]any, deps MCPDependencies) (model.ExecutionActionRequest, error) {
	if deps.Caller != nil {
		return executionActionRequestFromCapability(args, *deps.Caller)
	}
	ancestry, _ := deps.Execution.ObserveNativeProcessAncestry(os.Getpid())
	// 관측이 실패하면 ancestry가 비어 core mutation validation이 호출자의
	// process receipt를 신뢰하는 대신 fail-closed로 동작한다.
	return executionActionRequestFromMCPWithAncestry(args, ancestry)
}

// executionActionRequestFromCapability never attaches this server's ancestry:
// a bound capability is the caller proof. Omitted actor fields take the
// verified identity; supplied ones (stdio only) must match it in core Verify.
func executionActionRequestFromCapability(args map[string]any, caller model.VerifiedActor) (model.ExecutionActionRequest, error) {
	req, err := executionActionRequestFromMCPWithAncestry(args, nil)
	if err != nil {
		return req, err
	}
	supplied := false
	for _, field := range actorArguments {
		if _, present := args[field]; present {
			supplied = true
		}
	}
	if !supplied {
		req.Actor = caller.Identity
		req.Actor.ProcessAncestry = nil
		return req, nil
	}
	req.Actor.ProcessAncestry = nil
	if _, pid := args["session_pid"]; !pid && argmap.String(args, "session_started_at") == "" && argmap.String(args, "session_executable") == "" {
		req.Actor.SessionProcess = nil
	}
	return req, nil
}

func executionActionRequestFromMCPWithAncestry(args map[string]any, ancestry []model.NativeProcessReceipt) (model.ExecutionActionRequest, error) {
	pid := argmap.Int(args, "session_pid", 0)
	return model.ExecutionActionRequest{
		Action: argmap.String(args, "action"), ID: argmap.String(args, "id"),
		Actor: model.NativeActor{
			Host: argmap.String(args, "host"), SessionID: argmap.String(args, "session_id"), AgentID: argmap.String(args, "agent_id"),
			SessionProcess:  &model.NativeProcessReceipt{PID: pid, StartedAt: argmap.String(args, "session_started_at"), Executable: argmap.String(args, "session_executable")},
			ProcessAncestry: append([]model.NativeProcessReceipt(nil), ancestry...),
		},
		CWD:        argmap.String(args, "cwd"),
		Generation: uint64(argmap.Int64(args, "generation", 0)), ExpectedGeneration: uint64(argmap.Int64(args, "expected_generation", 0)), CompletionGeneration: uint64(argmap.Int64(args, "completion_generation", 0)),
		TokenFile: argmap.String(args, "claim_token_file"), ClaimCurrentToken: argmap.Bool(args, "claim_current_token"), ReplaceAction: argmap.String(args, "replace_action"),
		InventoryFingerprint: argmap.String(args, "inventory_fingerprint"), QuiescenceFingerprint: argmap.String(args, "quiescence_fingerprint"),
		Reason: argmap.String(args, "reason"), Preview: argmap.Bool(args, "preview"), Confirm: argmap.Bool(args, "confirm"),
		FinalHead: argmap.String(args, "final_head"), VerificationReportPath: argmap.String(args, "verification_report_path"),
		Verification: argmap.StringSlice(args, "verification"), RemoteArtifactURL: argmap.String(args, "remote_artifact_url"),
	}, nil
}
