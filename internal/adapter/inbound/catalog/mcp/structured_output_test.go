package mcp

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	docscontract "issueops/internal/contract/docs"
	inspectcontract "issueops/internal/contract/inspect"
	contract "issueops/internal/contract/mcp"
)

var readOnlyStructuredTools = []string{"harness_inspect", "docs_index"}

func TestOnlyStructuredToolsAreReadOnlyClosedWorld(t *testing.T) {
	for _, tool := range AdvertisedTools() {
		structured := slices.Contains(readOnlyStructuredTools, tool.Name)
		if !structured {
			if tool.Annotations != nil || tool.OutputSchema != nil {
				t.Errorf("%s must not carry annotations or an output schema: %+v", tool.Name, tool)
			}
			continue
		}
		a := tool.Annotations
		if a == nil || a.ReadOnlyHint == nil || !*a.ReadOnlyHint || a.OpenWorldHint == nil || *a.OpenWorldHint {
			t.Errorf("%s annotations = %+v, want readOnlyHint=true openWorldHint=false", tool.Name, a)
		}
		if a != nil && (a.DestructiveHint != nil || a.IdempotentHint != nil) {
			t.Errorf("%s must not claim destructive/idempotent hints: %+v", tool.Name, a)
		}
		if tool.OutputSchema == nil {
			t.Errorf("%s has no output schema", tool.Name)
		}
	}
}

func TestToolMapsCarryOutputSchemaAndAnnotations(t *testing.T) {
	byName := map[string]map[string]any{}
	for _, tool := range Build().Tools {
		byName[tool["name"].(string)] = tool
	}
	for _, name := range readOnlyStructuredTools {
		if _, ok := byName[name]["outputSchema"].(map[string]any); !ok {
			t.Errorf("%s map lacks outputSchema: %#v", name, byName[name])
		}
		if _, ok := byName[name]["annotations"].(*contract.ToolAnnotations); !ok {
			t.Errorf("%s map lacks annotations: %#v", name, byName[name])
		}
	}
	for name, tool := range byName {
		if slices.Contains(readOnlyStructuredTools, name) {
			continue
		}
		if _, ok := tool["outputSchema"]; ok {
			t.Errorf("%s must not advertise outputSchema", name)
		}
		if _, ok := tool["annotations"]; ok {
			t.Errorf("%s must not advertise annotations", name)
		}
	}
}

func TestBuildAdvertisesAuthorityFieldsOnWorkspaceToolsOnly(t *testing.T) {
	workspace := workspaceScopedToolNames()
	for _, tool := range Build().Tools {
		name := tool["name"].(string)
		properties, _ := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
		for _, field := range []string{"authority_file", "workspace_root", "cwd"} {
			_, has := properties[field]
			if want := slices.Contains(workspace, name); has != want {
				t.Errorf("%s field %s present=%v want %v", name, field, has, want)
			}
		}
	}
	for _, name := range workspace {
		if !slices.ContainsFunc(Build().Tools, func(tool map[string]any) bool { return tool["name"] == name }) {
			t.Errorf("workspace-scoped tool %q is not in the catalog", name)
		}
	}
}

func TestAuthorityFieldsKeepExistingProperties(t *testing.T) {
	for _, tool := range Build().Tools {
		if tool["name"] != "harness_inspect" {
			continue
		}
		properties := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
		if _, ok := properties["repo"]; !ok {
			t.Fatalf("harness_inspect lost repo: %#v", properties)
		}
		if _, ok := properties["authority_file"]; !ok {
			t.Fatalf("harness_inspect missing authority_file: %#v", properties)
		}
	}
	for _, tool := range contract.CoreProjectTools() {
		if tool.Name == "harness_inspect" {
			if _, ok := tool.InputSchema["properties"].(map[string]any)["authority_file"]; ok {
				t.Fatal("assembly mutated the contract tool schema")
			}
		}
	}
}

func resolvedOutputSchema(t *testing.T, name string) *jsonschema.Resolved {
	t.Helper()
	for _, tool := range AdvertisedTools() {
		if tool.Name != name {
			continue
		}
		raw, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema jsonschema.Schema
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		resolved, err := schema.Resolve(nil)
		if err != nil {
			t.Fatalf("%s output schema does not resolve: %v", name, err)
		}
		return resolved
	}
	t.Fatalf("tool %s not found", name)
	return nil
}

func validateAgainst(t *testing.T, resolved *jsonschema.Resolved, value any) error {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var instance any
	if err := json.Unmarshal(raw, &instance); err != nil {
		t.Fatal(err)
	}
	return resolved.Validate(instance)
}

func TestInspectOutputSchemaModelsInspectInfo(t *testing.T) {
	resolved := resolvedOutputSchema(t, "harness_inspect")
	observation := inspectcontract.Observation{Status: "verified", Features: []string{"tools"}}
	populated := inspectcontract.InspectInfo{
		OK: true, Skills: []inspectcontract.SkillInfo{{Name: "s", Path: "/p", HasSkillMD: true, Description: "d"}},
		Docs: []string{"a.md"},
		Integration: inspectcontract.IntegrationStatus{Hosts: []inspectcontract.HostIntegration{{
			Host: "codex", Installed: observation, Linked: observation, Configured: observation,
			Discovered: observation, Connected: observation, Protocol: observation,
		}}},
	}
	for name, value := range map[string]any{"zero value (null slices)": inspectcontract.InspectInfo{}, "populated": populated} {
		if err := validateAgainst(t, resolved, value); err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
	}
	for name, value := range map[string]any{
		"string ok":     map[string]any{"ok": "yes"},
		"missing field": map[string]any{"ok": true},
		"extra field":   mustMergeInspect(t, map[string]any{"surprise": 1}),
	} {
		if err := validateAgainst(t, resolved, value); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	assertSchemaCoversStruct(t, "harness_inspect", reflect.TypeOf(inspectcontract.InspectInfo{}))
}

func mustMergeInspect(t *testing.T, extra map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(inspectcontract.InspectInfo{})
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func TestDocsIndexOutputSchemaModelsDocsIndexResult(t *testing.T) {
	resolved := resolvedOutputSchema(t, "docs_index")
	populated := docscontract.DocsIndexResult{OK: true, Docs: []docscontract.DocIndexInfo{{RelPath: "a.md", Path: "/a.md", Title: "A", Headings: []string{"h"}, Bytes: 12}, {RelPath: "b.md"}}}
	for name, value := range map[string]any{"zero value (null slices)": docscontract.DocsIndexResult{}, "populated": populated} {
		if err := validateAgainst(t, resolved, value); err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
	}
	if err := validateAgainst(t, resolved, map[string]any{"ok": true, "docs": "none"}); err == nil {
		t.Error("malformed docs_index output was accepted")
	}
	assertSchemaCoversStruct(t, "docs_index", reflect.TypeOf(docscontract.DocsIndexResult{}))
}

// assertSchemaCoversStruct walks the DTO and requires the output schema to
// name exactly its JSON fields, so a DTO change cannot drift from the schema.
func assertSchemaCoversStruct(t *testing.T, tool string, dto reflect.Type) {
	t.Helper()
	var schema map[string]any
	for _, candidate := range AdvertisedTools() {
		if candidate.Name == tool {
			schema = candidate.OutputSchema
		}
	}
	defs, _ := schema["$defs"].(map[string]any)
	walkStruct(t, tool, schema, defs, dto)
}

func walkStruct(t *testing.T, path string, schema, defs map[string]any, typ reflect.Type) {
	t.Helper()
	schema = deref(schema, defs)
	properties, _ := schema["properties"].(map[string]any)
	required, _ := schema["required"].([]string)
	seen := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name, opts, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		seen[name] = true
		property, ok := properties[name].(map[string]any)
		if !ok {
			t.Errorf("%s: schema lacks property %q", path, name)
			continue
		}
		if mustBeRequired := !strings.Contains(opts, "omitempty"); mustBeRequired != slices.Contains(required, name) {
			t.Errorf("%s.%s required=%v want %v", path, name, slices.Contains(required, name), mustBeRequired)
		}
		elem := field.Type
		if elem.Kind() == reflect.Slice {
			elem = elem.Elem()
			if !slices.Contains(typeNames(property["type"]), "null") {
				t.Errorf("%s.%s: slice property must allow null", path, name)
			}
			property, _ = property["items"].(map[string]any)
		}
		if elem.Kind() == reflect.Struct {
			walkStruct(t, path+"."+name, property, defs, elem)
		}
	}
	for name := range properties {
		if !seen[name] {
			t.Errorf("%s: schema property %q has no DTO field", path, name)
		}
	}
	if schema["additionalProperties"] != false {
		t.Errorf("%s: additionalProperties must be false", path)
	}
}

func deref(schema, defs map[string]any) map[string]any {
	ref, _ := schema["$ref"].(string)
	if ref == "" {
		return schema
	}
	target, _ := defs[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
	return target
}

func typeNames(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	}
	return nil
}

// workspaceScopedToolNames returns the names of the tools that advertise the
// request authority inputs.
func workspaceScopedToolNames() []string {
	names := make([]string, 0, len(workspaceScopedTools))
	for name := range workspaceScopedTools {
		names = append(names, name)
	}
	return names
}
