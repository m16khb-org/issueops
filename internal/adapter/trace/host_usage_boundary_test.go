package trace

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tracecontract "issueops/internal/contract/trace"
)

func TestHostUsageRedactsSecretsInMetadataLabels(t *testing.T) {
	secret := "sk-" + strings.Repeat("a", 24)
	body := fmt.Sprintf(
		"{\"type\":\"system\",\"claude_code_version\":%q}\n"+
			"{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"session_id\":\"s\",\"modelUsage\":{%q:{\"provider\":%q,\"inputTokens\":1,\"outputTokens\":2}}}\n",
		secret, secret, secret,
	)
	_, usage := decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(body))
	if len(usage.Samples) != 1 {
		t.Fatalf("samples=%d", len(usage.Samples))
	}
	requireMetrics(t, usage.Samples[0], i64(1), i64(2), nil, nil)
	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatal("usage labels exposed a known credential pattern")
	}
}

func TestClaudeMalformedErrorFlagDoesNotCertifyMeasuredZero(t *testing.T) {
	for _, flag := range []string{`"true"`, `null`, `1`, `{}`} {
		t.Run(flag, func(t *testing.T) {
			body := fmt.Sprintf(`{"type":"result","subtype":"success","is_error":%s,"session_id":"s","modelUsage":{"m":{"inputTokens":0,"outputTokens":0}}}`, flag)
			_, usage := decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(body))
			if len(usage.Samples) != 1 {
				t.Fatalf("samples=%d", len(usage.Samples))
			}
			sample := usage.Samples[0]
			if sample.Finality == tracecontract.UsageFinalityFinal || sample.InputTokens != nil || sample.OutputTokens != nil {
				t.Fatalf("malformed status certified a successful zero: %+v", sample)
			}
			if !hasWarning(usage.Warnings, "usage_invalid_status") {
				t.Fatalf("missing invalid-status warning: %v", usage.Warnings)
			}
		})
	}
}
