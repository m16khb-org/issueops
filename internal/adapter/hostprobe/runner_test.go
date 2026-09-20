package hostprobe

import (
	"errors"
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
