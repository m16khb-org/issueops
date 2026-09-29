package mcpcli

import (
	"context"
	"encoding/json"
	app "issueops/internal/application/gates"
	model "issueops/internal/contract/gates"
	policy "issueops/internal/contract/policy"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP SDK transport 세션을 통해 gates 도구 5종이 advertise되고 실제로
// 동작하는지 검증한다. handler-map 유닛 테스트(mcp_tool_gates_test.go)와
// 달리 이 테스트는 tools/list 스키마 검증과 실세션 round-trip을 잠근다.
func TestServeMCPStreamAdvertisesAndRunsGatesTools(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	session := startMCPTransportTestSession(t, "stdio", MCPDependencies{Catalog: testMCPCatalog(), Gates: testGatesService()})

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	advertised := map[string]bool{}
	for _, tool := range tools.Tools {
		advertised[tool.Name] = true
	}
	for _, name := range []string{"gates_init", "gates_check", "gates_status", "gates_report", "gates_abandon"} {
		if !advertised[name] {
			t.Fatalf("tools/list missing %s", name)
		}
	}

	dir := t.TempDir()
	ledger := filepath.Join(dir, "GATES.md")

	initResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "gates_init",
		Arguments: map[string]any{
			"file":  ledger,
			"scope": "sdk smoke",
			"gates": []any{"G1: sdk gate | CHECK: printf %s sdk-ok | EXPECT: sdk-ok", "G2: manual sdk"},
		},
	})
	if err != nil || initResult.IsError {
		t.Fatalf("gates_init over SDK failed: err=%v result=%+v", err, initResult)
	}

	checkResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "gates_check",
		Arguments: map[string]any{
			"workspace_root": dir, "cwd": dir, "files": []any{ledger},
			"env_allowlist": []any{"PATH"},
		},
	})
	if err != nil || checkResult.IsError {
		t.Fatalf("gates_check over SDK failed: err=%v result=%+v", err, checkResult)
	}
	content := toolResultText(checkResult)
	if !strings.Contains(content, `"complete": false`) || !strings.Contains(content, `"total_met": 1`) {
		t.Fatalf("gates_check SDK payload unexpected:\n%s", content)
	}

	updated, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "- [x] G1: sdk gate") || !strings.Contains(string(updated), "EVIDENCE: sdk-ok") {
		t.Fatalf("SDK gates_check did not update the ledger:\n%s", string(updated))
	}

	abandonResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "gates_abandon",
		Arguments: map[string]any{"file": ledger, "gate_id": "G2", "reason": "verified in SDK smoke"},
	})
	if err != nil || abandonResult.IsError {
		t.Fatalf("gates_abandon over SDK failed: err=%v result=%+v", err, abandonResult)
	}

	finalResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "gates_status",
		Arguments: map[string]any{"workspace_root": dir, "cwd": dir, "files": []any{ledger}},
	})
	if err != nil || finalResult.IsError {
		t.Fatalf("gates_status over SDK failed: err=%v result=%+v", err, finalResult)
	}
	if content := toolResultText(finalResult); !strings.Contains(content, `"complete": true`) {
		t.Fatalf("gates_status must report complete after abandon:\n%s", content)
	}
}

func toolResultText(result *mcp.CallToolResult) string {
	parts := make([]string, 0, len(result.Content))
	for _, item := range result.Content {
		if text, ok := item.(*mcp.TextContent); ok && text.Text != "" {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func TestGatesSDKServersKeepTheirOwnPolicyRunner(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	calls := [2]int{}
	sessions := make([]*mcp.ClientSession, 2)
	for i, root := range roots {
		service := testGatesService()
		service.Runner = app.CommandRunner{Evaluate: func(policy.CommandPolicyRequest) policy.CommandPolicyEvaluation {
			return policy.CommandPolicyEvaluation{Allowed: i == 1, DenyReasons: []string{"instance denied"}}
		}, Execute: func(policy.CommandPolicyRequest) policy.CommandRunResult {
			calls[i]++
			return policy.CommandRunResult{Stdout: "ok"}
		}}
		_, err := service.Init(model.InitRequest{File: filepath.Join(root, "GATES.md"), Scope: "scope", Gates: []string{"G1: proof | CHECK: printf ok | EXPECT: ok"}})
		if err != nil {
			t.Fatal(err)
		}
		sessions[i] = startMCPTransportTestSession(t, "stdio", MCPDependencies{Catalog: testMCPCatalog(), Gates: service})
	}
	for i, session := range sessions {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "gates_check", Arguments: map[string]any{"workspace_root": roots[i], "cwd": roots[i]}})
		if err != nil || result.IsError {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		var check model.CheckResult
		if err := json.Unmarshal([]byte(toolResultText(result)), &check); err != nil {
			t.Fatal(err)
		}
		if check.Complete != (i == 1) || len(check.Files) != 1 || len(check.Files[0].Gates) != 1 || check.Files[0].Gates[0].PolicyDenied != (i == 0) {
			t.Fatalf("server %d result=%+v", i, check)
		}
		data, err := os.ReadFile(filepath.Join(roots[i], "GATES.md"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "- [x] G1") != (i == 1) {
			t.Fatalf("server %d wrote wrong ledger: %s", i, data)
		}
	}
	if calls != [2]int{0, 1} {
		t.Fatalf("executions=%v", calls)
	}
}
