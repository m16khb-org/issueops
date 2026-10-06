package mcpcli

import (
	"fmt"
	"sort"
	"strings"

	mcpcontract "issueops/internal/contract/mcp"
)

const (
	argAuthorityFile = "authority_file"
	argWorkspaceRoot = "workspace_root"
	argCWD           = "cwd"
)

// actorArguments name a native caller. A capability already proves the caller,
// so shared HTTP rejects them next to authority_file.
var actorArguments = []string{"host", "session_id", "agent_id", "session_pid", "session_started_at", "session_executable"}

type toolScope uint8

const (
	toolScopeUnclassified toolScope = iota
	// toolScopeServer tools read only the installed root or fixed user state.
	// The bearer is their whole authorization.
	toolScopeServer
	// toolScopeWorkspace tools act on a caller workspace. Shared HTTP requires a
	// capability and a resolved request scope for them.
	toolScopeWorkspace
)

type RecordKind string

const (
	recordNone     RecordKind = ""
	RecordIssueOps RecordKind = "issueops"
	RecordLoop     RecordKind = "loop"
)

type toolAuthority struct {
	scope toolScope
	// optional workspace tools stay server-scoped when no root input is present.
	optional bool
	// rootArgs name the workspace; they must agree with workspace_root.
	rootArgs []string
	// pathArgs are request-relative files resolved against the request cwd.
	pathArgs []string
	// record resolves the root from the stored record named by "id".
	record RecordKind
}

var (
	serverTool    = toolAuthority{scope: toolScopeServer}
	workspaceTool = toolAuthority{scope: toolScopeWorkspace}
	repoTool      = toolAuthority{scope: toolScopeWorkspace, rootArgs: []string{"repo"}}
	loopRecord    = toolAuthority{scope: toolScopeWorkspace, record: RecordLoop}
)

// mcpToolAuthorities classifies every MCP tool explicitly. Annotations never
// decide authority; an unlisted tool fails shared HTTP registration.
var mcpToolAuthorities = map[string]toolAuthority{
	// project
	"harness_inspect":             {scope: toolScopeWorkspace, optional: true, rootArgs: []string{"repo"}},
	"atomic_commit_preflight":     {scope: toolScopeWorkspace, rootArgs: []string{"path"}},
	"commit_policy":               serverTool,
	"skill_manifest":              serverTool,
	"docs_index":                  serverTool,
	"project_docs_route":          repoTool,
	"project_docs_bootstrap_plan": repoTool,
	"project_docs_read":           repoTool,
	"project_docs_revise":         repoTool,
	"project_docs_append":         repoTool,
	"api_doc_review":              {scope: toolScopeWorkspace, rootArgs: []string{"repo"}, pathArgs: []string{"diff_file", "prompt_file"}},
	"api_doc_static_check":        repoTool,
	// policy_state
	"command_policy_check": workspaceTool,
	"command_fake_run":     workspaceTool,
	"command_policy_audit": workspaceTool,
	"state_write":          serverTool,
	"state_read":           serverTool,
	"state_list":           serverTool,
	"state_prune":          serverTool,
	"state_doctor":         serverTool,
	"state_maintain":       serverTool,
	// issueops
	"issueops_execution": {scope: toolScopeWorkspace, record: RecordIssueOps, pathArgs: []string{"claim_token_file", "issue_snapshot_file", "verification_report_path"}},
	// loop
	"loop_start":          repoTool,
	"loop_record_attempt": loopRecord,
	"loop_status":         loopRecord,
	"loop_stop":           loopRecord,
	// gates
	"gates_init":    {scope: toolScopeWorkspace, pathArgs: []string{"file"}},
	"gates_abandon": {scope: toolScopeWorkspace, pathArgs: []string{"file"}},
	"gates_check":   {scope: toolScopeWorkspace, pathArgs: []string{"files"}},
	"gates_status":  {scope: toolScopeWorkspace, pathArgs: []string{"files"}},
	"gates_report":  {scope: toolScopeWorkspace, pathArgs: []string{"files"}},
	// channel
	"channel_send": serverTool,
	"channel_recv": serverTool,
	// assistant_worker
	"contract_schema":      serverTool,
	"contract_check":       serverTool,
	"web_fetch_resilient":  serverTool,
	"worker_enqueue":       serverTool,
	"worker_status":        serverTool,
	"worker_list":          serverTool,
	"worker_cancel":        serverTool,
	"commit_suggest":       repoTool,
	"lint_diagnose":        repoTool,
	"worker_run_read_only": workspaceTool,
	// self_loop
	"self_augment":           serverTool,
	"self_augment_lesson":    serverTool,
	"self_verify":            serverTool,
	"self_verify_candidates": serverTool,
	"self_verify_history":    serverTool,
	"self_verify_compare":    serverTool,
	"self_verify_promote":    serverTool,
}

func validateHTTPToolClassification(catalog mcpcontract.Catalog) error {
	names := map[string]bool{}
	for _, tool := range catalog.Tools {
		name, _ := tool["name"].(string)
		names[name] = true
	}
	for name := range catalog.Dispatch {
		names[name] = true
	}
	var missing []string
	for name := range names {
		if mcpToolAuthorities[name].scope == toolScopeUnclassified {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("mcp http: tools without an authority class: %s", strings.Join(missing, ", "))
	}
	return nil
}
