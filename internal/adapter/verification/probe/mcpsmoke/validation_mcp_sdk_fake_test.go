package mcpsmoke

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	selfverify "issueops/internal/contract/selfverify"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const fakeServerModeEnv = "ISSUEOPS_MCPSMOKE_FAKE_SERVER_MODE"

// TestMain lets the test binary double as a minimal MCP server so the SDK
// smoke path runs hermetically: runSDKSmoke launches `<binary> mcp`.
func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeServerModeEnv); mode != "" && len(os.Args) == 2 && os.Args[1] == "mcp" {
		os.Exit(runFakeServer(mode))
	}
	os.Exit(m.Run())
}

func runFakeServer(mode string) int {
	server := mcp.NewServer(&mcp.Implementation{Name: "fake-issueops", Version: "1"}, nil)
	for _, name := range []string{"project_docs_route", "state_prune", "state_doctor"} {
		failing := mode == "tool-error" && name == "state_prune"
		server.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}},
			func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				if failing {
					return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "boom"}}}, nil
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok " + req.Params.Name}}}, nil
			})
	}
	uris := []string{
		"issueops://commit-policy", "issueops://state", "issueops://docs",
		"issueops://command-policy", "issueops://project-docs", "issueops://project-doc-upkeep",
	}
	if mode == "missing-resource" {
		uris = uris[:3]
	}
	for _, uri := range uris {
		server.AddResource(&mcp.Resource{URI: uri, Name: uri}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, Text: "body of " + req.Params.URI}}}, nil
		})
	}
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		return 1
	}
	return 0
}

func runFakeSmoke(t *testing.T, mode string) selfverify.StepResult {
	t.Helper()
	got := runSDKSmoke(t.TempDir(), mustExecutable(t), []string{fakeServerModeEnv + "=" + mode}, 30*time.Second)
	if !got.OK {
		if got.Stdout != "" {
			t.Fatalf("failed step must not carry stdout: %+v", got)
		}
		if got.StderrBytes != len(got.Stderr) {
			t.Fatalf("StderrBytes mismatch: %+v", got)
		}
	}
	return got
}

func TestRunSDKSmokeSuccessTranscriptHasElevenJSONResults(t *testing.T) {
	step := runFakeSmoke(t, "ok")
	if !step.OK || step.Error != "" {
		t.Fatalf("smoke failed: %+v", step)
	}
	lines := splitLines(step.Stdout)
	if len(lines) != 11 {
		t.Fatalf("want 11 transcript lines (init, tools, 6 resources, 3 tool calls), got %d", len(lines))
	}
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("line %d is not JSON: %q", i+1, line)
		}
	}
	for _, want := range []string{"fake-issueops", "project_docs_route", "body of issueops://project-doc-upkeep", "ok state_doctor"} {
		if !strings.Contains(step.Stdout, want) {
			t.Fatalf("transcript missing %q", want)
		}
	}
}

func TestRunSDKSmokeReportsUnreadableResource(t *testing.T) {
	step := runFakeSmoke(t, "missing-resource")
	if step.OK || !strings.HasPrefix(step.Error, "read MCP resource issueops://command-policy: ") {
		t.Fatalf("unexpected step: %+v", step)
	}
}

func TestRunSDKSmokeReportsToolIsError(t *testing.T) {
	step := runFakeSmoke(t, "tool-error")
	if step.OK || step.Error != "MCP tool state_prune returned isError" {
		t.Fatalf("unexpected step: %+v", step)
	}
}

func mustExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}
