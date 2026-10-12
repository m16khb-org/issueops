package trace

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tracecontract "issueops/internal/contract/trace"
)

// 첫 줄이 빈 줄이어도 이후 줄은 각각 따로 해석되어야 한다.
func TestHostUsageSplitsLinesAfterLeadingBlankLine(t *testing.T) {
	body := "\n" + omoEnd("one", `{"input":3,"output":1}`) + "\n" + omoEnd("two", `{"input":4,"output":2}`) + "\n"
	input, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(body))
	if len(usage.Samples) != 2 || input.Incomplete || hasWarning(input.Warnings, "invalid_jsonl_line") {
		t.Fatalf("leading blank line broke line splitting: input=%+v samples=%d", input.Warnings, len(usage.Samples))
	}
}

// 한도(256바이트) 정확히인 provider/model 라벨은 유지하고, 넘으면 비운다.
func TestHostUsageLabelLengthLimit(t *testing.T) {
	for _, tc := range []struct {
		length int
		keep   bool
	}{{256, true}, {257, false}} {
		t.Run(fmt.Sprint(tc.length), func(t *testing.T) {
			value := strings.Repeat("m", tc.length)
			line := fmt.Sprintf(`{"type":"message_end","message":{"role":"assistant","responseId":"r","provider":%q,"model":%q,"usage":{"input":1,"output":1}}}`, value, value)
			_, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(line+"\n"))
			if len(usage.Samples) != 1 {
				t.Fatalf("samples=%d", len(usage.Samples))
			}
			sample := usage.Samples[0]
			want := ""
			if tc.keep {
				want = value
			}
			if sample.Provider != want || sample.Model != want {
				t.Fatalf("provider/model len = %d/%d, want %d", len(sample.Provider), len(sample.Model), len(want))
			}
		})
	}
}

// claude_code_version은 256바이트까지만 sample에 실린다.
func TestHostUsageClaudeVersionLengthLimit(t *testing.T) {
	result := `{"type":"result","subtype":"success","is_error":false,"session_id":"s","uuid":"u","modelUsage":{"m":{"inputTokens":1,"outputTokens":1}}}`
	for _, tc := range []struct {
		length int
		keep   bool
	}{{256, true}, {257, false}} {
		t.Run(fmt.Sprint(tc.length), func(t *testing.T) {
			version := strings.Repeat("9", tc.length)
			body := fmt.Sprintf(`{"type":"system","claude_code_version":%q}`, version) + "\n" + result + "\n"
			_, usage := decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(body))
			if len(usage.Samples) != 1 {
				t.Fatalf("samples=%d", len(usage.Samples))
			}
			want := ""
			if tc.keep {
				want = version
			}
			if got := usage.Samples[0].Version; got != want {
				t.Fatalf("version len = %d, want %d", len(got), len(want))
			}
		})
	}
}

// 최종 결과 없이 usage만 보이는 중간 이벤트만 usage_final_missing을 만든다.
func TestHostUsageIntermediateUsageDetection(t *testing.T) {
	for _, tc := range []struct {
		name    string
		format  string
		line    string
		missing bool
	}{
		{"claude top-level usage", tracecontract.InputFormatClaudeJSON, `{"type":"assistant","usage":{"input_tokens":5}}`, true},
		{"claude message usage", tracecontract.InputFormatClaudeJSON, `{"type":"assistant","message":{"usage":{"input_tokens":5}}}`, true},
		{"claude no usage", tracecontract.InputFormatClaudeJSON, `{"type":"assistant","message":{"role":"assistant"}}`, false},
		{"omo message_update usage", tracecontract.InputFormatOmoJSON, `{"type":"message_update","usage":{"input":5}}`, true},
		{"omo message_update no usage", tracecontract.InputFormatOmoJSON, `{"type":"message_update"}`, false},
		{"omo turn_end top-level usage", tracecontract.InputFormatOmoJSON, `{"type":"turn_end","usage":{"input":5}}`, true},
		{"omo turn_end message usage", tracecontract.InputFormatOmoJSON, `{"type":"turn_end","message":{"usage":{"input":5}}}`, true},
		{"omo turn_end no usage", tracecontract.InputFormatOmoJSON, `{"type":"turn_end"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, usage := decodeHost(t, tc.format, []byte(tc.line+"\n"))
			if got := hasWarning(usage.Warnings, "usage_final_missing"); got != tc.missing {
				t.Fatalf("usage_final_missing=%v want %v (warnings=%v)", got, tc.missing, usage.Warnings)
			}
		})
	}
}

// 파일 상한을 넘기면 줄 경계에서 자른다. 유일한 개행이 맨 앞이어도 그 줄까지는 남는다.
func TestHostLoadTruncationKeepsLeadingBlankLine(t *testing.T) {
	path := t.TempDir() + "/lead.jsonl"
	payload := append([]byte("\n"), make([]byte, tracecontract.HostInputMaxBytes)...)
	for i := 1; i < len(payload); i++ {
		payload[i] = 'a'
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	_, body, truncated, err := Source{}.Load(path, tracecontract.InputFormatOmoJSON)
	if err != nil || !truncated || string(body) != "\n" {
		t.Fatalf("truncated=%v err=%v bodyLen=%d", truncated, err, len(body))
	}
}
