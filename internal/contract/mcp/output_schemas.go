package mcp

const jsonSchema2020 = "https://json-schema.org/draft/2020-12/schema"

// ReadOnlyClosedWorldAnnotations marks a tool that neither mutates state nor
// reaches outside the local installation. Only tools whose whole behavior is a
// read may use it.
func ReadOnlyClosedWorldAnnotations() *ToolAnnotations {
	readOnly, closedWorld := true, false
	return &ToolAnnotations{ReadOnlyHint: &readOnly, OpenWorldHint: &closedWorld}
}

func schemaString() map[string]any  { return map[string]any{"type": "string"} }
func schemaBool() map[string]any    { return map[string]any{"type": "boolean"} }
func schemaInteger() map[string]any { return map[string]any{"type": "integer"} }

// schemaArray models a Go slice: a nil slice serializes as null.
func schemaArray(items map[string]any) map[string]any {
	return map[string]any{"type": []string{"array", "null"}, "items": items}
}

func schemaRef(name string) map[string]any { return map[string]any{"$ref": "#/$defs/" + name} }

// schemaObject describes a DTO whose JSON fields are exactly the properties.
func schemaObject(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

// InspectOutputSchema models inspect.InspectInfo, the harness_inspect result.
func InspectOutputSchema() map[string]any {
	observation := schemaObject(map[string]any{
		"status":              map[string]any{"type": "string", "description": "verified, failed, unknown, or not_checked."},
		"source":              schemaString(),
		"observed_at":         schemaString(),
		"reason":              schemaString(),
		"host_version":        schemaString(),
		"requested_revision":  schemaString(),
		"negotiated_revision": schemaString(),
		"config_sha256":       schemaString(),
		"features":            schemaArray(schemaString()),
	}, "status", "source", "observed_at", "reason", "host_version", "requested_revision", "negotiated_revision", "config_sha256", "features")
	host := schemaObject(map[string]any{
		"host":        schemaString(),
		"config_path": schemaString(),
		"transport":   schemaString(),
		"skill_path":  schemaString(),
		"installed":   schemaRef("observation"),
		"linked":      schemaRef("observation"),
		"configured":  schemaRef("observation"),
		"discovered":  schemaRef("observation"),
		"connected":   schemaRef("observation"),
		"protocol":    schemaRef("observation"),
	}, "host", "config_path", "transport", "skill_path", "installed", "linked", "configured", "discovered", "connected", "protocol")
	skill := schemaObject(map[string]any{
		"name":            schemaString(),
		"path":            schemaString(),
		"has_skill_md":    schemaBool(),
		"has_openai_yaml": schemaBool(),
		"description":     schemaString(),
	}, "name", "path", "has_skill_md", "has_openai_yaml")
	integration := schemaObject(map[string]any{
		"codex_skill_path":          schemaString(),
		"codex_skill_installed":     schemaBool(),
		"codex_mcp_configured":      schemaBool(),
		"claude_skill_path":         schemaString(),
		"claude_skill_installed":    schemaBool(),
		"project_claude_skill_path": schemaString(),
		"project_claude_skill":      schemaBool(),
		"project_claude_mcp_config": schemaBool(),
		"mcp_binary_path":           schemaString(),
		"hosts":                     schemaArray(schemaRef("host")),
	}, "codex_skill_path", "codex_skill_installed", "codex_mcp_configured", "claude_skill_path", "claude_skill_installed",
		"project_claude_skill_path", "project_claude_skill", "project_claude_mcp_config", "mcp_binary_path", "hosts")
	schema := schemaObject(map[string]any{
		"ok":            schemaBool(),
		"version":       schemaString(),
		"issueops_root": schemaString(),
		"target_repo":   schemaString(),
		"skills":        schemaArray(schemaRef("skill")),
		"docs":          schemaArray(schemaString()),
		"integration":   schemaRef("integration"),
		"generated_at":  schemaString(),
	}, "ok", "version", "issueops_root", "target_repo", "skills", "docs", "integration", "generated_at")
	schema["$schema"] = jsonSchema2020
	schema["$defs"] = map[string]any{"observation": observation, "host": host, "skill": skill, "integration": integration}
	return schema
}

// DocsIndexOutputSchema models docs.DocsIndexResult, the docs_index result.
func DocsIndexOutputSchema() map[string]any {
	doc := schemaObject(map[string]any{
		"rel_path": schemaString(),
		"path":     schemaString(),
		"title":    schemaString(),
		"headings": schemaArray(schemaString()),
		"bytes":    schemaInteger(),
	}, "rel_path", "path", "title", "headings", "bytes")
	schema := schemaObject(map[string]any{
		"ok":            schemaBool(),
		"version":       schemaString(),
		"issueops_root": schemaString(),
		"docs":          schemaArray(schemaRef("doc")),
		"generated_at":  schemaString(),
	}, "ok", "version", "issueops_root", "docs", "generated_at")
	schema["$schema"] = jsonSchema2020
	schema["$defs"] = map[string]any{"doc": doc}
	return schema
}
