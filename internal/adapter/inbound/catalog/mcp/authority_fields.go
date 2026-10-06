package mcp

import "maps"

const (
	authorityFileField = "authority_file"
	workspaceRootField = "workspace_root"
	cwdField           = "cwd"
)

// workspaceScopedTools act on a caller workspace, so shared HTTP requires a
// capability for them and the catalog advertises the request authority inputs.
// cmd/issueops/mcpcli classifies the same tools in mcpToolAuthorities; its
// tests fail when the two sets drift.
var workspaceScopedTools = map[string]bool{
	"harness_inspect":             true,
	"atomic_commit_preflight":     true,
	"project_docs_route":          true,
	"project_docs_bootstrap_plan": true,
	"project_docs_read":           true,
	"project_docs_revise":         true,
	"project_docs_append":         true,
	"api_doc_review":              true,
	"api_doc_static_check":        true,
	"command_policy_check":        true,
	"command_fake_run":            true,
	"command_policy_audit":        true,
	"issueops_execution":          true,
	"loop_start":                  true,
	"loop_record_attempt":         true,
	"loop_status":                 true,
	"loop_stop":                   true,
	"gates_init":                  true,
	"gates_abandon":               true,
	"gates_check":                 true,
	"gates_status":                true,
	"gates_report":                true,
	"commit_suggest":              true,
	"lint_diagnose":               true,
	"worker_run_read_only":        true,
}

var authorityFieldDescriptions = map[string]string{
	authorityFileField: "Managed credential file printed by `issueops mcp authorize`. Required for this tool on the shared HTTP server.",
	workspaceRootField: "Absolute workspace root for this request. Must agree with repo/path and the stored record.",
	cwdField:           "Request working directory inside the workspace. Relative file arguments resolve against it; defaults to the workspace root.",
}

// withAuthorityFields advertises the request authority inputs on every
// workspace tool. The tool maps are copied, never mutated in place.
func withAuthorityFields(tools []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		schema, ok := tool["inputSchema"].(map[string]any)
		if !workspaceScopedTools[name] || !ok {
			out = append(out, tool)
			continue
		}
		properties, _ := schema["properties"].(map[string]any)
		properties = maps.Clone(properties)
		if properties == nil {
			properties = map[string]any{}
		}
		for field, description := range authorityFieldDescriptions {
			if _, exists := properties[field]; !exists {
				properties[field] = map[string]any{"type": "string", "description": description}
			}
		}
		schema = maps.Clone(schema)
		schema["properties"] = properties
		tool = maps.Clone(tool)
		tool["inputSchema"] = schema
		out = append(out, tool)
	}
	return out
}
