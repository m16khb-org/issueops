package basiccli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	trace "issueops/internal/contract/trace"
)

const (
	hostClaudeResult = `{"type":"result","subtype":"success","is_error":false,"session_id":"cli-sess","uuid":"cli-res","modelUsage":{"model-x":{"inputTokens":12,"outputTokens":3,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"costUSD":0.5}}}` + "\n"
	hostCodexTurn    = `{"type":"thread.started","thread_id":"cli-thread"}` + "\n" + `{"type":"turn.completed","usage":{"input_tokens":8,"cached_input_tokens":2,"output_tokens":1}}` + "\n"
	hostOmoMessage   = `{"type":"session","id":"cli-omo"}` + "\n" + `{"type":"message_end","message":{"role":"assistant","responseId":"cli-msg","stopReason":"stop","usage":{"input":5,"output":2,"cacheRead":0,"cacheWrite":1}}}` + "\n"
	hostOmpMessage   = `{"type":"session","version":3,"id":"cli-omp"}` + "\n" + `{"type":"message_end","message":{"role":"assistant","provider":"anthropic","model":"claude-haiku-4-5","responseId":"cli-msg","stopReason":"toolUse","usage":{"input":6,"output":2,"cacheRead":0,"cacheWrite":0,"totalTokens":8,"cost":{"total":0.001}}}}` + "\n"
)

func runTraceAnalyzeJSON(t *testing.T, args ...string) (trace.TraceAnalyzeResult, string, error) {
	t.Helper()
	out, err := captureTraceGuardPolicyStdout(t, func() error {
		return RunTrace(append([]string{"analyze"}, args...))
	})
	var result trace.TraceAnalyzeResult
	if strings.TrimSpace(out) != "" {
		if decodeErr := json.Unmarshal([]byte(out), &result); decodeErr != nil {
			t.Fatalf("decode trace analyze JSON: %v\n%s", decodeErr, out)
		}
	}
	return result, out, err
}

func TestRunTraceAnalyzeHostFormatsReportUsageThroughTheCLI(t *testing.T) {
	cases := []struct {
		format, body, host string
		input              int64
		coverage           string
	}{
		{trace.InputFormatClaudeJSON, hostClaudeResult, "claude", 12, trace.UsageCoverageComplete},
		{trace.InputFormatCodexExec, hostCodexTurn, "codex", 8, trace.UsageCoverageUnknown},
		{trace.InputFormatOmoJSON, hostOmoMessage, "omo", 5, trace.UsageCoverageComplete},
		{trace.InputFormatOmpJSON, hostOmpMessage, "omp", 6, trace.UsageCoverageComplete},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "export.jsonl")
			writeFileForCLITest(t, path, tc.body)
			result, out, err := runTraceAnalyzeJSON(t, "--input", path, "--input-format", tc.format, "--json")
			if err != nil || !result.OK || !result.Complete || result.Usage == nil || len(result.Usage.Samples) != 1 {
				t.Fatalf("result=%+v err=%v\n%s", result, err, out)
			}
			sample := result.Usage.Samples[0]
			if sample.Host != tc.host || sample.InputTokens == nil || *sample.InputTokens != tc.input || result.Usage.Coverage != tc.coverage {
				t.Errorf("sample=%+v coverage=%q", sample, result.Usage.Coverage)
			}
			if strings.Contains(out, "cli-sess") || strings.Contains(out, "cli-thread") || strings.Contains(out, "cli-msg") || !strings.Contains(out, `"cache_write_tokens":`) {
				t.Errorf("identifiers must be hashed and absent metrics serialized as null:\n%s", out)
			}
		})
	}
}

func TestRunTraceAnalyzeRejectsUnsupportedFormatAndStateKeyFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.jsonl")
	writeFileForCLITest(t, path, hostOmoMessage)
	result, _, err := runTraceAnalyzeJSON(t, "--input", path, "--input-format", "jsonl-ish", "--json")
	if err == nil || result.OK || result.Complete || len(result.Warnings) != 1 || result.Warnings[0] != "unsupported_input_format" {
		t.Errorf("unsupported format: result=%+v err=%v", result, err)
	}
	result, _, err = runTraceAnalyzeJSON(t, "--input", "some-state-key", "--input-format", trace.InputFormatOmoJSON, "--json")
	if err == nil || result.OK || len(result.Warnings) != 1 || result.Warnings[0] != "trace_read_error" {
		t.Errorf("host formats must not fall back to state keys: result=%+v err=%v", result, err)
	}
}

func TestRunTraceAnalyzeDefaultFormatOmitsUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.json")
	writeFileForCLITest(t, path, `{"summary":{"failed_steps":1,"failure_class":"deterministic","failed_step":"go test","rerun_commands":["go test ./..."]}}`)
	result, out, err := runTraceAnalyzeJSON(t, "--input", path, "--json")
	if err != nil || !result.OK || result.Usage != nil || strings.Contains(out, `"usage"`) || result.FindingCount != 1 {
		t.Errorf("default mode must not gain a usage field: result=%+v err=%v\n%s", result, err, out)
	}
}

func TestRunTraceAnalyzeTextOutputSummarizesUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.jsonl")
	writeFileForCLITest(t, path, hostCodexTurn)
	out := captureStatusVerifyStdout(t, func() error {
		return RunTraceAnalyze([]string{"--input", path, "--input-format", trace.InputFormatCodexExec})
	})
	if !strings.Contains(out, "usage: 1 sample(s), coverage=unknown") {
		t.Errorf("text output:\n%s", out)
	}
}
