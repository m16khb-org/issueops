package hostprobe

import (
	"context"
	"errors"
	"issueops/internal/port"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEpisodeRootSurfacesCleanupFailureAfterPermissionFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing", "episode")
	cleanupCalled := false
	_, err := newEpisodeRoot(normalizeDependencies(Dependencies{
		TempDir: func(string, string) (string, error) { return root, nil },
		RemoveAll: func(path string) error {
			cleanupCalled = true
			if path != root {
				t.Fatalf("cleanup path = %q", path)
			}
			return errors.New("sensitive cleanup detail")
		},
	}), "codex")
	if !cleanupCalled || !errors.Is(err, errPrivateRootCleanup) || episodeRootFailureCode(err) != "private_root_cleanup_failed" {
		t.Fatalf("cleanupCalled=%t err=%v code=%q", cleanupCalled, err, episodeRootFailureCode(err))
	}
}

func TestObservedModelFromOutputReadsOnlyStructuredModelFields(t *testing.T) {
	output := []byte("{\"type\":\"system\",\"subtype\":\"init\",\"model\":\"claude-opus-5\"}\n{\"model\":\"later\"}\n")
	if got := observedModelFromOutput(output); got != "claude-opus-5" {
		t.Fatalf("observed model=%q", got)
	}
	if got := observedModelFromOutput([]byte(`{"text":"model=secret"}`)); got != "" {
		t.Fatalf("freeform text produced model=%q", got)
	}
}

func TestBoundedBufferReportsTruncationWithoutShortWrite(t *testing.T) {
	buffer := &boundedBuffer{limit: 4}
	value := []byte("123456")
	written, err := buffer.Write(value)
	if err != nil || written != len(value) || buffer.String() != "1234" || !buffer.truncated {
		t.Fatalf("written=%d err=%v buffer=%q truncated=%t", written, err, buffer.String(), buffer.truncated)
	}
	if strings.Contains(buffer.String(), "56") {
		t.Fatal("bounded buffer retained truncated suffix")
	}
}

func TestSemanticResponseDigestIsHostNeutral(t *testing.T) {
	shapes := []any{
		map[string]any{"content": "captured"},
		map[string]any{"content": "captured", "duration_ms": 17, "server": "issueops_probe"},
		map[string]any{
			"content": []any{map[string]any{"type": "text", "text": "captured"}},
			"details": map[string]any{"server": "issueops_probe", "tool": "harness_probe_empty_object"},
		},
	}
	var want string
	for index, shape := range shapes {
		got, err := semanticResponseDigest([]any{shape})
		if err != nil {
			t.Fatalf("shape %d: %v", index, err)
		}
		if index == 0 {
			want = got
		} else if got != want {
			t.Fatalf("shape %d digest = %q, want %q", index, got, want)
		}
	}
	different, err := semanticResponseDigest([]any{map[string]any{"content": "different"}})
	if err != nil {
		t.Fatal(err)
	}
	if different == want {
		t.Fatal("semantic response difference produced the same digest")
	}
}

func TestObserveHostStreamCountsEveryToolCallAndObservedModel(t *testing.T) {
	tests := []struct {
		name   string
		stream string
	}{
		{
			name: "codex",
			stream: strings.Join([]string{
				`{"type":"thread.started","thread_id":"thread-probe","model":"gpt-5.4"}`,
				`{"type":"item.completed","item":{"type":"command_execution","id":"ambient","status":"completed","aggregated_output":"ignored","exit_code":0}}`,
				`{"type":"item.completed","item":{"type":"mcp_tool_call","id":"target","server":"issueops_probe","status":"completed","result":{"content":"captured"}}}`,
			}, "\n") + "\n",
		},
		{
			name: "claude",
			stream: strings.Join([]string{
				`{"type":"system","subtype":"init","model":"claude-opus-4-6"}`,
				`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"ambient","name":"Read","input":{}},{"type":"tool_use","id":"target","name":"mcp__issueops_probe__harness_probe_empty_object","input":{}}]}}`,
				`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"ambient","content":"ignored"},{"type":"tool_result","tool_use_id":"target","content":"captured"}]},"tool_use_result":{"content":"captured"}}`,
			}, "\n") + "\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := observeHostStream([]byte(test.stream))
			if err != nil {
				t.Fatal(err)
			}
			if got.Model == "" || got.AmbientToolCount != 2 || got.MCPCallCount != 1 || !validSHA256(got.ResponseSHA256) {
				t.Fatalf("observation = %+v", got)
			}
		})
	}
}

func TestObserveCodexHookTrustWarningIsNotATool(t *testing.T) {
	warning := `{"type":"item.completed","item":{"type":"error","message":"` + "`--dangerously-bypass-hook-trust` is enabled. Enabled hooks may run without review for this invocation." + `"}}` + "\n"
	got, err := observeHostStream([]byte(warning + warning + string(codexSuccessfulStream("gpt-5.6-sol"))))
	if err != nil || got.AmbientToolCount != 1 || got.MCPCallCount != 1 {
		t.Fatalf("observation=%+v err=%v", got, err)
	}
	if _, err := observeHostStream([]byte(`{"type":"item.completed","item":{"type":"error","message":"authentication failed"}}`)); err == nil {
		t.Fatal("real host error accepted")
	}
}

func TestObserveClaudeArrayResult(t *testing.T) {
	stream := `{"type":"system","subtype":"init","model":"claude-opus-5"}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"target","name":"mcp__issueops_probe__harness_probe_empty_object","input":{}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"target","content":[{"type":"text","text":"captured"}]}]},"tool_use_result":[{"type":"text","text":"captured"}]}`
	got, err := observeHostStream([]byte(stream))
	if err != nil || got.MCPCallCount != 1 || !validSHA256(got.ResponseSHA256) {
		t.Fatalf("observation=%+v err=%v", got, err)
	}
	for _, errorValue := range []string{`"false"`, `true`} {
		invalid := strings.Replace(stream, `"tool_use_id":"target","content"`, `"tool_use_id":"target","is_error":`+errorValue+`,"content"`, 1)
		if _, err := observeHostStream([]byte(invalid)); err == nil {
			t.Fatal("invalid or failed tool result accepted")
		}
	}
}

func TestObserveFailedSessionStartIsNotSuccess(t *testing.T) {
	for _, failure := range []string{`"exit_code":1,"outcome":"error"`, `"exit_code":0,"outcome":"error"`} {
		_, err := observeHostStream([]byte(`{"type":"system","subtype":"hook_response","hook_event":"SessionStart",` + failure + `}`))
		if err == nil {
			t.Fatal("failed hook accepted")
		}
	}
}

func TestRecordedSessionStartCanUseStreamModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observation.json")
	if err := os.WriteFile(path+".hooks", []byte(`{"event":"SessionStart"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	recorded, err := observeRecordedHookEvents(path)
	if err != nil || !recorded.SessionStartObserved || recorded.Model != "" {
		t.Fatalf("recorded=%+v err=%v", recorded, err)
	}
	observation := hostStreamObservation{Model: "claude-opus-5"}
	if err := mergeHookObservation(&observation, recorded); err != nil || !observation.SessionStartObserved || observation.Model != "claude-opus-5" {
		t.Fatalf("merge=%+v err=%v", observation, err)
	}
}

func TestRunnersRequirePrivateSessionStartReceipt(t *testing.T) {
	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			request := codexRequest("gpt-5.6-sol")
			if host == "claude" {
				request = claudeProbeRequest()
			}
			deps := Dependencies{LookPath: func(string) (string, error) { return "/test/" + host, nil }, Process: &codexFakeProcess{run: func(_ context.Context, cmd CommandRequest) (CommandOutput, error) {
				var stream []byte
				if host == "codex" {
					writeCodexCapture(t, filepath.Join(cmd.Cwd, "result.json"), request.RunToken)
					stream = codexSuccessfulStream(request.Model)
				} else {
					writeClaudeCapture(t, filepath.Join(cmd.Cwd, "result.json"), request.RunToken)
					stream = claudeSuccessfulStream(request.Model, request.ProbeTool)
				}
				stream = append([]byte("{\"type\":\"system\",\"subtype\":\"hook_response\",\"hook_event\":\"SessionStart\",\"exit_code\":0,\"outcome\":\"success\"}\n"), stream...)
				return CommandOutput{Stdout: stream}, nil
			}}}
			var runner port.HostProbeRunner = NewCodexRunner("issueops", deps)
			if host == "claude" {
				runner = NewClaudeRunner("issueops", deps)
			}
			got := runner.Run(context.Background(), request)
			if got.Completed || got.Code != "hook_observation_invalid" {
				t.Fatalf("missing private marker accepted: %+v", got)
			}
		})
	}
}

func TestObserveSessionStartRequiresCompleteSuccessReceipt(t *testing.T) {
	for _, fields := range []string{"", `,"exit_code":0`, `,"outcome":"success"`} {
		_, err := observeHostStream([]byte(`{"type":"system","subtype":"hook_response","hook_event":"SessionStart"` + fields + `}`))
		if err == nil {
			t.Fatalf("incomplete hook success receipt accepted: %s", fields)
		}
	}
}

func TestSemanticResponseRejectsToolFailure(t *testing.T) {
	for _, key := range []string{"isError", "is_error"} {
		_, err := semanticResponseDigest([]any{map[string]any{"content": "failed", key: true}})
		if err == nil {
			t.Fatalf("tool failure accepted through %s", key)
		}
	}
}
