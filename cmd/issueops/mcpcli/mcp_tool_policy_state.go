package mcpcli

import (
	"context"
	"time"

	"issueops/cmd/issueops/mcpcli/argmap"
	policydomain "issueops/internal/contract/policy"
)

func commandPolicyRequestFromArgs(args map[string]any) policydomain.CommandPolicyRequest {
	return policydomain.CommandPolicyRequest{
		WorkspaceRoot:  argmap.String(args, "workspace_root"),
		CWD:            argmap.String(args, "cwd"),
		Argv:           argmap.StringSlice(args, "argv"),
		Timeout:        argmap.StringDefault(args, "timeout", "30s"),
		EnvAllowlist:   argmap.StringSlice(args, "env_allowlist"),
		NetworkAllowed: argmap.Bool(args, "network_allowed"),
		WriteAllowed:   argmap.Bool(args, "write_allowed"),
		ShellAllowed:   argmap.Bool(args, "shell_allowed"),
		ShellReason:    argmap.String(args, "shell_reason"),
		AuditLogID:     argmap.String(args, "audit_log_id"),
	}
}

// mcpToolResult는 err가 없으면 결과를 payload로, 있으면 주어진 코드와 메시지의 실패로 변환한다.
func mcpToolResult(result any, err error, code int64, message string) MCPToolOutcome {
	if err != nil {
		return mcpToolFailure(newProtocolError(code, message, err.Error()))
	}
	return mcpToolPayload(result)
}

func handlePolicyStateMCPToolCall(ctx context.Context, call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	state := deps.State
	switch call.Name {
	case "command_policy_check":
		return mcpToolPayload(deps.Policy.Evaluate(commandPolicyRequestFromArgs(call.Arguments)))
	case "command_fake_run":
		return mcpToolPayload(deps.Policy.FakeRun(commandPolicyRequestFromArgs(call.Arguments)))
	case "command_policy_audit":
		result, err := deps.Audit.Audit(commandPolicyRequestFromArgs(call.Arguments))
		return mcpToolResult(result, err, -32000, "command_policy_audit failed")
	case "state_write":
		result, err := state.Write(ctx, argmap.String(call.Arguments, "key"), argmap.String(call.Arguments, "content"))
		return mcpToolResult(result, err, -32602, "State write failed")
	case "state_read":
		result, err := state.Read(argmap.String(call.Arguments, "key"))
		return mcpToolResult(result, err, -32602, "State read failed")
	case "state_list":
		result, err := state.List()
		return mcpToolResult(result, err, -32000, "State list failed")
	case "state_prune":
		maxAge, err := time.ParseDuration(argmap.String(call.Arguments, "max_age"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State prune failed", "invalid max_age: "+err.Error()))
		}
		result, err := state.Prune(ctx, maxAge, argmap.Bool(call.Arguments, "confirm"))
		return mcpToolResult(result, err, -32602, "State prune failed")
	case "state_doctor":
		result, err := state.Doctor()
		return mcpToolResult(result, err, -32000, "State doctor failed")
	case "state_maintain":
		result, err := state.Maintain()
		return mcpToolResult(result, err, -32000, "State maintain failed")
	default:
		return MCPToolOutcome{}
	}
}
