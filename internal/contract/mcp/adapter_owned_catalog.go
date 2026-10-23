package mcp

func AdapterOwnedTools() []Tool {
	return []Tool{
		{
			Name:        "contract_schema",
			Annotations: ReadOnlyClosedWorldAnnotations(),
			Description: "Return the current CLI/MCP compatibility contract, required response fields, and stable schema hash. This tool is read-only and is used before changing response DTOs.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "contract_check",
			Annotations: ReadOnlyClosedWorldAnnotations(),
			Description: "Check whether the current CLI/MCP compatibility contract is internally consistent. This tool is read-only and returns a pass/fail result plus warnings.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "worker_enqueue",
			Annotations: LocalWriteAnnotations(true, false),
			Description: "Create a no-shell local worker job record. This writes only to the issueops worker state directory and never executes shell commands.",
			InputSchema: map[string]any{"type": "object", "required": []string{"kind"}, "properties": map[string]any{"kind": map[string]any{"type": "string"}, "payload": map[string]any{"type": "string"}}},
		},
		{
			Name:        "worker_run_read_only",
			Annotations: LocalWriteAnnotations(true, false),
			Description: "Run an argv-only read-only command as a local worker evidence job. This writes only the worker job record and command evidence, forces write/network/shell disabled, applies workspace policy, timeout, env allowlist, redaction, and bounded output.",
			InputSchema: map[string]any{"type": "object", "required": []string{"kind", "workspace_root", "cwd", "argv"}, "properties": map[string]any{
				"kind":           map[string]any{"type": "string"},
				"payload":        map[string]any{"type": "string"},
				"workspace_root": map[string]any{"type": "string"},
				"cwd":            map[string]any{"type": "string"},
				"argv":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"timeout":        map[string]any{"type": "string"},
				"env_allowlist":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}},
		},
		{
			Name:        "worker_status",
			Annotations: ReadOnlyClosedWorldAnnotations(),
			Description: "Read a no-shell local worker job by id. This is read-only and returns the persisted job lifecycle record.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}},
		},
		{
			Name:        "worker_list",
			Annotations: ReadOnlyClosedWorldAnnotations(),
			Description: "List no-shell local worker jobs from the issueops worker state directory. This is read-only.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			Name:        "worker_cancel",
			Annotations: LocalWriteAnnotations(false, true),
			Description: "Mark a queued no-shell local worker job as cancelled. This writes only the job lifecycle record and never kills processes.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}},
		},
	}
}
