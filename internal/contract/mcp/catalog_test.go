package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func allTools() map[string][]Tool {
	return map[string][]Tool{
		"core":            CoreProjectTools(),
		"adapter_owned":   AdapterOwnedTools(),
		"channel":         ChannelTools(),
		"command_policy":  CommandPolicyTools(),
		"policy_audit":    CommandPolicyAuditTools(),
		"gates":           GatesTools(),
		"issueops":        IssueOpsBasicTools(),
		"local_assistant": LocalAssistantTools(),
		"loop":            LoopTools(),
		"state":           StateTools(),
		"self_loop":       SelfLoopAdvertisedTools(),
	}
}

func TestToolNamesAreUniqueAcrossCatalogsAndDescribed(t *testing.T) {
	t.Parallel()
	seen := map[string]string{}
	for catalog, tools := range allTools() {
		if len(tools) == 0 {
			t.Errorf("%s catalog is empty", catalog)
		}
		for _, tool := range tools {
			if tool.Name == "" || strings.TrimSpace(tool.Name) != tool.Name || strings.ContainsAny(tool.Name, " -") {
				t.Errorf("%s tool name %q must be a non-empty snake_case token", catalog, tool.Name)
			}
			if strings.TrimSpace(tool.Description) == "" {
				t.Errorf("tool %q has an empty description", tool.Name)
			}
			if previous, dup := seen[tool.Name]; dup {
				t.Errorf("tool %q is registered in both %s and %s", tool.Name, previous, catalog)
			}
			seen[tool.Name] = catalog
		}
	}
}

// checkObjectSchema walks a JSON Schema fragment and verifies every object
// schema's required keys are declared in its properties and every $ref
// resolves against defs.
func checkObjectSchema(t *testing.T, path string, schema map[string]any, defs map[string]any) {
	t.Helper()
	if ref, ok := schema["$ref"].(string); ok {
		name, found := strings.CutPrefix(ref, "#/$defs/")
		if !found {
			t.Errorf("%s: unsupported $ref %q", path, ref)
			return
		}
		if _, ok := defs[name]; !ok {
			t.Errorf("%s: $ref %q has no definition", path, ref)
		}
		return
	}
	props, _ := schema["properties"].(map[string]any)
	if required, ok := schema["required"]; ok {
		keys, ok := required.([]string)
		if !ok {
			t.Fatalf("%s: required has type %T, want []string", path, required)
		}
		for _, key := range keys {
			if _, declared := props[key]; !declared {
				t.Errorf("%s: required key %q missing from properties", path, key)
			}
		}
	}
	for name, raw := range props {
		if child, ok := raw.(map[string]any); ok {
			checkObjectSchema(t, path+"."+name, child, defs)
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		checkObjectSchema(t, path+"[]", items, defs)
	}
}

func TestToolSchemasAreObjectSchemasWithDeclaredRequiredKeys(t *testing.T) {
	t.Parallel()
	for catalog, tools := range allTools() {
		for _, tool := range tools {
			if tool.InputSchema["type"] != "object" {
				t.Errorf("%s/%s: input schema type = %v, want object", catalog, tool.Name, tool.InputSchema["type"])
			}
			defs, _ := tool.InputSchema["$defs"].(map[string]any)
			checkObjectSchema(t, tool.Name+".input", tool.InputSchema, defs)
			if tool.OutputSchema != nil {
				if tool.OutputSchema["type"] != "object" {
					t.Errorf("%s/%s: output schema type = %v, want object", catalog, tool.Name, tool.OutputSchema["type"])
				}
				outDefs, _ := tool.OutputSchema["$defs"].(map[string]any)
				checkObjectSchema(t, tool.Name+".output", tool.OutputSchema, outDefs)
			}
			if _, err := json.Marshal(tool); err != nil {
				t.Errorf("%s: tool does not serialize for tools/list: %v", tool.Name, err)
			}
		}
	}
}

func TestResourcesHaveUniqueIssueOpsURIsAndMimeTypes(t *testing.T) {
	t.Parallel()
	seenURI, seenName := map[string]bool{}, map[string]bool{}
	for _, r := range Resources() {
		if !strings.HasPrefix(r.URI, "issueops://") || len(r.URI) == len("issueops://") {
			t.Errorf("resource URI %q must be issueops://<path>", r.URI)
		}
		if seenURI[r.URI] || seenName[r.Name] {
			t.Errorf("duplicate resource uri/name: %q / %q", r.URI, r.Name)
		}
		seenURI[r.URI], seenName[r.Name] = true, true
		if strings.TrimSpace(r.Name) == "" || strings.TrimSpace(r.Description) == "" {
			t.Errorf("resource %q needs name and description", r.URI)
		}
		if r.MimeType != "text/markdown" && r.MimeType != "application/json" {
			t.Errorf("resource %q has unexpected mime type %q", r.URI, r.MimeType)
		}
	}
}
