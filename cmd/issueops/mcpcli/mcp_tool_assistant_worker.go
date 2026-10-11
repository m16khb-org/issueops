package mcpcli

import (
	"context"
	"time"

	"issueops/cmd/issueops/mcpcli/argmap"
	commitsuggestcontract "issueops/internal/contract/commitsuggest"
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	policy "issueops/internal/contract/policy"
	webfetchcontract "issueops/internal/contract/webfetch"
)

func handleAssistantWorkerMCPToolCall(ctx context.Context, call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	switch call.Name {
	case "commit_suggest":
		result, err := deps.Commit.Suggest(commitsuggestcontract.CommitSuggestRequest{
			RepoRoot: deps.resolveTarget(argmap.String(call.Arguments, "repo")),
			Staged:   argmap.Bool(call.Arguments, "staged"),
		})
		return mcpPayloadOrFailure(result, err, -32000, "commit_suggest failed")
	case "lint_diagnose":
		result, err := deps.Lint.Diagnose(lintdiagnosecontract.LintDiagnoseRequest{
			RepoRoot:    deps.resolveTarget(argmap.String(call.Arguments, "repo")),
			CommandArgv: argmap.StringSlice(call.Arguments, "command_argv"),
		})
		return mcpPayloadOrFailure(result, err, -32000, "lint_diagnose failed")
	case "web_fetch_resilient":
		return handleWebFetchTool(ctx, call, deps)
	case "contract_schema", "contract_check":
		return mcpToolPayload(deps.Compatibility())
	case "worker_enqueue":
		result, err := deps.Worker.Enqueue(ctx, argmap.String(call.Arguments, "kind"), argmap.String(call.Arguments, "payload"))
		return mcpPayloadOrFailure(result, err, -32000, "worker_enqueue failed")
	case "worker_run_read_only":
		return handleWorkerRunReadOnlyTool(ctx, call, deps)
	case "worker_status":
		result, err := deps.Worker.Read(argmap.String(call.Arguments, "id"))
		return mcpPayloadOrFailure(result, err, -32000, "worker_status failed")
	case "worker_list":
		result, err := deps.Worker.List()
		return mcpPayloadOrFailure(result, err, -32000, "worker_list failed")
	case "worker_cancel":
		result, err := deps.Worker.Cancel(ctx, argmap.String(call.Arguments, "id"))
		return mcpPayloadOrFailure(result, err, -32000, "worker_cancel failed")
	default:
		return MCPToolOutcome{}
	}
}

func handleWebFetchTool(ctx context.Context, call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	timeout, err := time.ParseDuration(argmap.StringDefault(call.Arguments, "timeout", "30s"))
	if err != nil {
		return mcpToolFailure(newProtocolError(-32602, "invalid timeout", err.Error()))
	}
	result, err := deps.Fetch(ctx, webfetchcontract.Request{
		URL:      argmap.String(call.Arguments, "url"),
		Timeout:  timeout,
		MaxChars: argmap.Int(call.Arguments, "max_chars", 0),
	})
	return mcpPayloadOrFailure(result, err, -32000, "web_fetch_resilient failed")
}

func handleWorkerRunReadOnlyTool(ctx context.Context, call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	result, err := deps.Worker.RunReadOnly(
		ctx,
		argmap.String(call.Arguments, "kind"),
		argmap.String(call.Arguments, "payload"),
		policy.CommandPolicyRequest{
			WorkspaceRoot: argmap.String(call.Arguments, "workspace_root"),
			CWD:           argmap.String(call.Arguments, "cwd"),
			Argv:          argmap.StringSlice(call.Arguments, "argv"),
			Timeout:       argmap.StringDefault(call.Arguments, "timeout", "30s"),
			EnvAllowlist:  argmap.StringSlice(call.Arguments, "env_allowlist"),
		},
	)
	return mcpPayloadOrFailure(result, err, -32000, "worker_run_read_only failed")
}
