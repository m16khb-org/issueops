package hostprobe

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runnerMutClaudeStream(userExtra string) string {
	return `{"type":"system","subtype":"init","model":"claude-opus-5-5"}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"target","name":"mcp__issueops_probe__harness_probe_empty_object","input":{}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"target","content":"captured"}]}` + userExtra + `}`
}

func TestExecRunnerOutputExactlyAtBoundIsNotTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", MaxOutputBytes)), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := (ExecRunner{}).Run(context.Background(), CommandRequest{Argv: []string{"/bin/cat", path}})
	if err != nil || got.StdoutTruncated || len(got.Stdout) != MaxOutputBytes {
		t.Fatalf("bytes=%d truncated=%t err=%v", len(got.Stdout), got.StdoutTruncated, err)
	}
}

func TestDecodeEpisodeCaptureAcceptsEvidenceExactlyAtBound(t *testing.T) {
	request := captureRequest()
	data := captureJSON(t, request, nil)
	padded := append(data, []byte(strings.Repeat(" ", MaxOutputBytes-len(data)))...)
	path := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(path, padded, 0o600); err != nil {
		t.Fatal(err)
	}
	capture, err := decodeEpisodeCapture(path, request)
	if err != nil || capture.CallCount != 1 {
		t.Fatalf("capture=%+v err=%v", capture, err)
	}
}

func TestObserveHostStreamAcceptsStreamExactlyAtBound(t *testing.T) {
	stream := string(codexSuccessfulStream("gpt-6-sol"))
	stream += strings.Repeat(" ", MaxOutputBytes-len(stream))
	got, err := observeHostStream([]byte(stream))
	if err != nil || got.MCPCallCount != 1 || got.Model != "gpt-6-sol" {
		t.Fatalf("observation=%+v err=%v", got, err)
	}
	if _, err := observeHostStream([]byte(stream + " ")); err == nil {
		t.Fatal("stream above bound accepted")
	}
}

func TestObserveHostStreamReadsModelFromMessage(t *testing.T) {
	stream := `{"type":"assistant","message":{"model":"claude-sonnet-5-5","content":[]}}
{"type":"assistant","message":{"model":"   ","content":[]}}`
	got, err := observeHostStream([]byte(stream))
	if err != nil || got.Model != "claude-sonnet-5-5" {
		t.Fatalf("observation=%+v err=%v", got, err)
	}
	blank, err := observeHostStream([]byte(`{"type":"assistant","message":{"model":"  ","content":[]}}`))
	if err != nil || blank.Model != "" {
		t.Fatalf("blank observation=%+v err=%v", blank, err)
	}
}

func TestObserveClaudeNullToolUseResultFallsBackToToolResultBlock(t *testing.T) {
	absent, err := observeHostStream([]byte(runnerMutClaudeStream("")))
	if err != nil {
		t.Fatal(err)
	}
	null, err := observeHostStream([]byte(runnerMutClaudeStream(`,"tool_use_result":null`)))
	if err != nil {
		t.Fatal(err)
	}
	present, err := observeHostStream([]byte(runnerMutClaudeStream(`,"tool_use_result":{"content":"other"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if !validSHA256(absent.ResponseSHA256) || null.ResponseSHA256 != absent.ResponseSHA256 {
		t.Fatalf("null digest %q != absent digest %q", null.ResponseSHA256, absent.ResponseSHA256)
	}
	if present.ResponseSHA256 == absent.ResponseSHA256 {
		t.Fatal("tool_use_result was ignored in favour of the tool_result block")
	}
}
