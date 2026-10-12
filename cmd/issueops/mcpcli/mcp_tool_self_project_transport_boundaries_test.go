package mcpcli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestProjectAndSelfToolFailuresReportInvalidParamsCode(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	absent := t.TempDir() + "/absent"
	tests := []struct {
		name    string
		self    bool
		call    MCPToolCall
		wantMsg string
	}{
		{"project route", false, MCPToolCall{Name: "project_docs_route", Arguments: map[string]any{"repo": absent}}, "Project docs route failed"},
		{"project bootstrap plan", false, MCPToolCall{Name: "project_docs_bootstrap_plan", Arguments: map[string]any{"repo": absent}}, "Project docs bootstrap plan failed"},
		{"project read", false, MCPToolCall{Name: "project_docs_read", Arguments: map[string]any{"repo": t.TempDir(), "rel_path": "../escape.md"}}, "Project docs read failed"},
		{"self history", true, MCPToolCall{Name: "self_verify_history", Arguments: map[string]any{"prune_retention": true}}, "Self-verify history failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var outcome MCPToolOutcome
			if tc.self {
				outcome = testHandleSelfLoopMCPToolCall(tc.call)
			} else {
				outcome = handleProjectMCPToolCall(tc.call, testProjectDependencies())
			}
			if !outcome.Handled || outcome.Err == nil || outcome.Err.Code != -32602 || outcome.Err.Message != tc.wantMsg {
				t.Fatalf("outcome = %+v, want handled -32602 %q", outcome, tc.wantMsg)
			}
		})
	}
}

func TestHTTPAccessLogRecordsToolOutcomePerResultKind(t *testing.T) {
	t.Setenv("ISSUEOPS_WORKER_DIR", t.TempDir())
	server := startHTTPTestServer(t, testTransportServices(), nil)
	tests := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"worker_list", map[string]any{}, "tool=worker_list outcome=ok"},
		{"project_docs_route", map[string]any{"repo": t.TempDir()}, "tool=project_docs_route outcome=tool_error"},
		{"worker_status", map[string]any{"id": "../bad"}, "tool=worker_status outcome=protocol_error"},
	}
	for _, tc := range tests {
		if _, _, _, err := server.callTool(t.Context(), revisionInitialize, tc.tool, tc.args, nil); err != nil {
			t.Fatalf("%s: %v", tc.tool, err)
		}
	}
	log := server.access.String()
	for _, tc := range tests {
		if !strings.Contains(log, tc.want) {
			t.Errorf("access log missing %q:\n%s", tc.want, log)
		}
	}
}

func TestServeMCPStreamWritesSessionDiagnosticsToProvidedWriter(t *testing.T) {
	const input = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"diag","version":"0"}}}` + "\n{bad\n"
	var diagnostics bytes.Buffer
	_ = ServeMCPStreamWithDependencies(strings.NewReader(input), io.Discard, &diagnostics, testTransportServices())
	if !strings.Contains(diagnostics.String(), "server session ended with error") {
		t.Fatalf("diagnostics = %q, want the session failure", diagnostics.String())
	}
}
