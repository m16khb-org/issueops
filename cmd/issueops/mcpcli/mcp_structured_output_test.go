package mcpcli

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	docscontract "issueops/internal/contract/docs"
	inspectcontract "issueops/internal/contract/inspect"
	mcpcontract "issueops/internal/contract/mcp"
)

const rawPayloadSentinel = "RAW-PAYLOAD-SENTINEL-7731"

var structuredToolNames = []string{"harness_inspect", "docs_index"}

func structuredToolDeps(inspect func(string, string) any) MCPDependencies {
	deps := testTransportServices()
	deps.Inspect = inspect
	deps.Resources.DocsIndex = func(root, version string) docscontract.DocsIndexResult {
		return docscontract.DocsIndexResult{OK: true, Version: version, IssueOpsRoot: root}
	}
	return deps
}

func nilSliceInspect(string, string) any {
	return inspectcontract.InspectInfo{OK: true, Version: "test"}
}

func resolveListedSchema(t *testing.T, listed any) *jsonschema.Resolved {
	t.Helper()
	encoded, err := json.Marshal(listed)
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(encoded, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("listed output schema does not resolve: %v", err)
	}
	return resolved
}

func listedTool(t *testing.T, tools []*mcp.Tool, name string) *mcp.Tool {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %s not listed", name)
	return nil
}

func assertStructuredToolOverSession(t *testing.T, session *mcp.ClientSession) {
	t.Helper()
	listed, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range structuredToolNames {
		tool := listedTool(t, listed.Tools, name)
		if tool.OutputSchema == nil || tool.Annotations == nil || !tool.Annotations.ReadOnlyHint ||
			tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Fatalf("%s listing = outputSchema %v annotations %+v", name, tool.OutputSchema, tool.Annotations)
		}
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
		if err != nil || result.IsError {
			t.Fatalf("%s call: result=%+v err=%v", name, result, err)
		}
		structured, ok := result.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("%s structuredContent = %#v", name, result.StructuredContent)
		}
		var fromText map[string]any
		if err := json.Unmarshal([]byte(toolResultText(result)), &fromText); err != nil || !reflect.DeepEqual(fromText, structured) {
			t.Fatalf("%s structuredContent differs from text: text=%v structured=%v err=%v", name, fromText, structured, err)
		}
		if err := resolveListedSchema(t, tool.OutputSchema).Validate(structured); err != nil {
			t.Fatalf("%s structuredContent violates the listed schema: %v", name, err)
		}
		if value, present := structured["docs"]; name == "docs_index" && (!present || value != nil) {
			t.Fatalf("docs_index nil slice must be null: %#v", structured)
		}
		if value, present := structured["skills"]; name == "harness_inspect" && (!present || value != nil) {
			t.Fatalf("harness_inspect nil slice must be null: %#v", structured)
		}
	}
	for _, tool := range listed.Tools {
		if slices.Contains(structuredToolNames, tool.Name) {
			continue
		}
		if tool.Annotations != nil || tool.OutputSchema != nil {
			t.Fatalf("%s must carry neither annotations nor outputSchema: %+v", tool.Name, tool)
		}
	}
}

func TestSDKStructuredContentOverStdioAndStream(t *testing.T) {
	for _, mode := range []string{"stdio", "stream_conn"} {
		t.Run(mode, func(t *testing.T) {
			assertStructuredToolOverSession(t, startMCPTransportTestSession(t, mode, structuredToolDeps(nilSliceInspect)))
		})
	}
}

func TestSDKToolsListAgreesWithCatalog(t *testing.T) {
	catalog := mcpcatalog.Build()
	session := startMCPTransportTestSession(t, "stdio", structuredToolDeps(nilSliceInspect))
	listed, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != len(catalog.Tools) {
		t.Fatalf("listed %d tools, catalog has %d", len(listed.Tools), len(catalog.Tools))
	}
	for _, want := range catalog.Tools {
		name := want["name"].(string)
		got := listedTool(t, listed.Tools, name)
		for key, listedValue := range map[string]any{"inputSchema": got.InputSchema, "outputSchema": got.OutputSchema} {
			if !sameJSON(t, listedValue, want[key]) {
				t.Errorf("%s %s differs between catalog and tools/list", name, key)
			}
		}
		wantAnnotations, has := want["annotations"].(*mcpcontract.ToolAnnotations)
		if has != (got.Annotations != nil) || (has && !sameHints(wantAnnotations, got.Annotations)) {
			t.Errorf("%s annotations differ: listed=%+v catalog=%+v", name, got.Annotations, wantAnnotations)
		}
	}
}

func sameHints(want *mcpcontract.ToolAnnotations, got *mcp.ToolAnnotations) bool {
	flag := func(hint *bool) bool { return hint != nil && *hint }
	return flag(want.ReadOnlyHint) == got.ReadOnlyHint && flag(want.IdempotentHint) == got.IdempotentHint &&
		(want.DestructiveHint == nil) == (got.DestructiveHint == nil) && (want.OpenWorldHint == nil) == (got.OpenWorldHint == nil) &&
		(want.OpenWorldHint == nil || *want.OpenWorldHint == *got.OpenWorldHint)
}

func sameJSON(t *testing.T, a, b any) bool {
	t.Helper()
	normalize := func(value any) any {
		if value == nil || reflect.ValueOf(value).Kind() == reflect.Pointer && reflect.ValueOf(value).IsNil() {
			return nil
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var out any
		if err := json.Unmarshal(encoded, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	return reflect.DeepEqual(normalize(a), normalize(b))
}

func TestSDKInvalidStructuredOutputIsProtocolInternalErrorWithoutPayload(t *testing.T) {
	session := startMCPTransportTestSession(t, "stdio", structuredToolDeps(func(string, string) any {
		return map[string]any{"ok": rawPayloadSentinel}
	}))
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "harness_inspect", Arguments: map[string]any{}})
	var protocolErr *jsonrpc.Error
	if !errors.As(err, &protocolErr) || protocolErr.Code != jsonrpc.CodeInternalError {
		t.Fatalf("invalid output must be a protocol internal error: result=%+v err=%v", result, err)
	}
	if strings.Contains(err.Error(), rawPayloadSentinel) || strings.Contains(string(protocolErr.Data), rawPayloadSentinel) {
		t.Fatalf("protocol error leaked the raw payload: %v", err)
	}
}

func TestSDKHarnessInspectForwardsHostReceiptsArgument(t *testing.T) {
	var gotRepo, gotReceipts string
	session := startMCPTransportTestSession(t, "stdio", structuredToolDeps(func(repo, receipts string) any {
		gotRepo, gotReceipts = repo, receipts
		return inspectcontract.InspectInfo{OK: true}
	}))
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "harness_inspect", Arguments: map[string]any{"repo": "/r", "host_receipts": "/receipts.json"}})
	if err != nil || result.IsError || gotRepo != "/r" || gotReceipts != "/receipts.json" {
		t.Fatalf("host_receipts not forwarded: repo=%q receipts=%q result=%+v err=%v", gotRepo, gotReceipts, result, err)
	}
}

func TestSDKStructuredToolErrorResultKeepsTextWithoutStructuredContent(t *testing.T) {
	handler := sdkToolHandlerWithContext(mcpcatalog.Build(), func(context.Context, MCPToolCall) MCPToolOutcome {
		return mcpToolErrorPayload(map[string]any{"ok": false, "error": "boom"})
	}, "harness_inspect")
	result, err := handler(t.Context(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)}})
	if err != nil || !result.IsError || result.StructuredContent != nil || !strings.Contains(toolResultText(result), "boom") {
		t.Fatalf("tool error result = %+v err=%v", result, err)
	}
}

func TestSDKDirectMarkdownHasNoStructuredContent(t *testing.T) {
	deps := structuredToolDeps(nilSliceInspect)
	want, err := deps.Resources.ReadHarnessFile(".issueops", "COMMIT_POLICY.md")
	if err != nil {
		t.Fatal(err)
	}
	session := startMCPTransportTestSession(t, "stdio", deps)
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "commit_policy", Arguments: map[string]any{}})
	if err != nil || result.IsError || result.StructuredContent != nil || toolResultText(result) != want {
		t.Fatalf("commit_policy must stay exact Markdown: result=%+v err=%v", result, err)
	}
}

func TestWorkspaceAuthorityClassificationMatchesCatalogAuthorityFields(t *testing.T) {
	var classified []string
	for name, authority := range mcpToolAuthorities {
		if authority.scope == toolScopeWorkspace {
			classified = append(classified, name)
		}
	}
	var advertised []string
	for _, tool := range mcpcatalog.Build().Tools {
		properties, _ := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
		if _, ok := properties["authority_file"]; ok {
			advertised = append(advertised, tool["name"].(string))
		}
	}
	slices.Sort(classified)
	slices.Sort(advertised)
	if !slices.Equal(classified, advertised) {
		t.Fatalf("workspace tools drifted:\nclassification=%v\ncatalog=%v", classified, advertised)
	}
}

func TestHTTPToolsListAndCallCarryStructuredOutput(t *testing.T) {
	for _, revision := range []string{revisionLegacy, revisionNew} {
		t.Run(revision, func(t *testing.T) {
			server := startHTTPTestServer(t, withFakeAuthority(structuredToolDeps(nilSliceInspect), nil, nil), nil)
			listed := httpResult(t, server, revision, "tools/list", map[string]any{}, "")
			var list struct {
				Tools []map[string]any `json:"tools"`
			}
			if err := json.Unmarshal(listed, &list); err != nil {
				t.Fatal(err)
			}
			byName := map[string]map[string]any{}
			for _, tool := range list.Tools {
				byName[tool["name"].(string)] = tool
			}
			for _, name := range structuredToolNames {
				annotations, _ := byName[name]["annotations"].(map[string]any)
				if annotations["readOnlyHint"] != true || annotations["openWorldHint"] != false || byName[name]["outputSchema"] == nil {
					t.Fatalf("%s http listing = %v", name, byName[name])
				}
				var call mcp.CallToolResult
				if err := json.Unmarshal(httpResult(t, server, revision, "tools/call", map[string]any{"name": name, "arguments": map[string]any{}}, name), &call); err != nil {
					t.Fatal(err)
				}
				structured, ok := call.StructuredContent.(map[string]any)
				var fromText map[string]any
				if !ok || json.Unmarshal([]byte(toolResultText(&call)), &fromText) != nil || !reflect.DeepEqual(fromText, structured) {
					t.Fatalf("%s http structuredContent=%#v text=%s", name, call.StructuredContent, toolResultText(&call))
				}
				if err := resolveListedSchema(t, byName[name]["outputSchema"]).Validate(structured); err != nil {
					t.Fatalf("%s http structuredContent violates schema: %v", name, err)
				}
			}
			for name, tool := range byName {
				if slices.Contains(structuredToolNames, name) {
					continue
				}
				if _, has := tool["annotations"]; has {
					t.Fatalf("%s advertises annotations over http", name)
				}
				if _, has := tool["outputSchema"]; has {
					t.Fatalf("%s advertises outputSchema over http", name)
				}
			}
		})
	}
}

func httpResult(t *testing.T, server httpTestServer, revision, method string, params map[string]any, name string) json.RawMessage {
	t.Helper()
	if revision >= revisionNew {
		params["_meta"] = map[string]any{
			"io.modelcontextprotocol/protocolVersion":    revision,
			"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "http-test", "version": "1"},
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		}
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 9, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	header := map[string]string{}
	if revision >= revisionNew {
		header["Mcp-Method"] = method
		if name != "" {
			header["Mcp-Name"] = name
		}
	}
	response, err := server.post(t.Context(), revision, string(body), header)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	message, err := readSSEResponse(response.Body)
	if err != nil || message.Error != nil {
		t.Fatalf("%s %s: message=%+v err=%v", revision, method, message, err)
	}
	return message.Result
}
