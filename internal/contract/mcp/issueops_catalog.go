package mcp

// IssueOpsBasicTools is the complete public IssueOps v1 MCP surface.
func IssueOpsBasicTools() []Tool {
	return []Tool{{
		Name:        "issueops_execution",
		Annotations: UnclaimedWriteAnnotations(),
		Description: "Prepare, inspect, claim, release, replace, reconcile, or complete the single IssueOps v1 execution lease. The action selects one shared CLI/MCP DTO; mutations require the exact native actor, generation, canonical cwd, and explicit confirmation.",
		InputSchema: issueOpsExecutionSchema(),
	}}
}

func issueOpsExecutionSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"action", "id"},
		"properties": map[string]any{
			"action":                   map[string]any{"type": "string", "enum": []string{"prepare", "status", "claim", "release", "replace", "reconcile", "complete"}},
			"id":                       map[string]any{"type": "string"},
			"host":                     map[string]any{"type": "string", "enum": []string{"codex", "claude", "omo", "omp", "opencode"}},
			"session_id":               map[string]any{"type": "string"},
			"agent_id":                 map[string]any{"type": "string"},
			"session_pid":              map[string]any{"type": "integer", "minimum": 1},
			"session_started_at":       map[string]any{"type": "string"},
			"session_executable":       map[string]any{"type": "string"},
			"cwd":                      map[string]any{"type": "string"},
			"generation":               map[string]any{"type": "integer", "minimum": 1},
			"expected_generation":      map[string]any{"type": "integer", "minimum": 1},
			"completion_generation":    map[string]any{"type": "integer", "minimum": 1},
			"claim_current_token":      map[string]any{"type": "boolean"},
			"claim_token_file":         map[string]any{"type": "string"},
			"replace_action":           map[string]any{"type": "string", "enum": []string{"preview", "revoke", "finalize-preview", "finalize", "reseed"}},
			"inventory_fingerprint":    map[string]any{"type": "string"},
			"quiescence_fingerprint":   map[string]any{"type": "string"},
			"reason":                   map[string]any{"type": "string"},
			"preview":                  map[string]any{"type": "boolean"},
			"confirm":                  map[string]any{"type": "boolean"},
			"final_head":               map[string]any{"type": "string"},
			"verification_report_path": map[string]any{"type": "string"},
			"verification":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"remote_artifact_url":      map[string]any{"type": "string"},
		},
	}
}
