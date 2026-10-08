package trace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	statecontract "issueops/internal/contract/state"
	tracecontract "issueops/internal/contract/trace"
	tracedomain "issueops/internal/domain/trace"
)

func digest(kind, id string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + id))
	return hex.EncodeToString(sum[:])
}

func scopeDigest(host, session string) string { return digest("scope", host+"\x00"+session) }

func i64(v int64) *int64 { return &v }

func samePtr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func hasWarning(warnings []string, want string) bool {
	return slices.Contains(warnings, want)
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "host-usage", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func decodeHost(t *testing.T, format string, body []byte) (tracedomain.Input, *tracecontract.UsageReport) {
	t.Helper()
	input := Source{}.Decode(body, format)
	if input.Usage == nil {
		t.Fatalf("format %s produced no usage report: %+v", format, input)
	}
	return input, input.Usage
}

func requireMetrics(t *testing.T, sample tracecontract.UsageSample, in, out, cacheRead, cacheWrite *int64) {
	t.Helper()
	if !samePtr(sample.InputTokens, in) || !samePtr(sample.OutputTokens, out) || !samePtr(sample.CacheReadTokens, cacheRead) || !samePtr(sample.CacheWriteTokens, cacheWrite) {
		t.Errorf("metrics = in:%v out:%v cacheRead:%v cacheWrite:%v", deref(sample.InputTokens), deref(sample.OutputTokens), deref(sample.CacheReadTokens), deref(sample.CacheWriteTokens))
	}
}

func deref(v *int64) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprint(*v)
}

func omoEnd(id, usage string) string {
	return fmt.Sprintf(`{"type":"message_end","message":{"role":"assistant","responseId":%q,"stopReason":"stop","usage":%s}}`, id, usage)
}

func TestHostUsageClaudeCumulativeSnapshotAndReset(t *testing.T) {
	// Given a stream whose modelUsage snapshots grow and then reset
	input, usage := decodeHost(t, tracecontract.InputFormatClaudeJSON, fixture(t, "claude-cumulative-reset.jsonl"))
	// Then only the last snapshot of each epoch survives and nothing is summed
	if len(usage.Samples) != 2 || input.Incomplete {
		t.Fatalf("samples=%d incomplete=%v usage=%+v", len(usage.Samples), input.Incomplete, usage)
	}
	first, second := usage.Samples[0], usage.Samples[1]
	requireMetrics(t, first, i64(250), i64(120), i64(30), i64(0))
	requireMetrics(t, second, i64(20), i64(5), i64(0), i64(0))
	if first.Epoch != "0" || second.Epoch != "1" {
		t.Errorf("epochs = %q, %q", first.Epoch, second.Epoch)
	}
	if first.Host != "claude" || first.Version != "2.1.0" || first.Model != "claude-x" {
		t.Errorf("identity fields = %+v", first)
	}
	if first.Temporality != tracecontract.UsageTemporalityCumulative || first.Finality != tracecontract.UsageFinalityFinal {
		t.Errorf("temporality/finality = %q/%q", first.Temporality, first.Finality)
	}
	if first.CostUSD == nil || *first.CostUSD != 0.03 || first.CostBasis != tracecontract.UsageCostBasisHostEstimate {
		t.Errorf("cost = %v basis=%q", first.CostUSD, first.CostBasis)
	}
	if first.SessionID != digest("session", "sess-claude-A") || first.ScopeID != scopeDigest("claude", "sess-claude-A") || first.MessageID != digest("message", "res-2") {
		t.Errorf("digests = %+v", first)
	}
	if usage.Coverage != tracecontract.UsageCoveragePartial || !hasWarning(usage.Warnings, "usage_cumulative_reset") {
		t.Errorf("coverage=%q warnings=%q", usage.Coverage, usage.Warnings)
	}
}

func TestHostUsageClaudeRepeatedIdenticalSnapshotIsNotDoubled(t *testing.T) {
	line := `{"type":"result","subtype":"success","is_error":false,"session_id":"s","uuid":"u1","modelUsage":{"m":{"inputTokens":9,"outputTokens":2,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"costUSD":0.5}}}`
	_, usage := decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(line+"\n"+line+"\n"))
	if len(usage.Samples) != 1 || usage.Coverage != tracecontract.UsageCoverageComplete {
		t.Fatalf("usage = %+v", usage)
	}
	requireMetrics(t, usage.Samples[0], i64(9), i64(2), i64(0), i64(0))
}

func TestHostUsageClaudeMissingSessionKeepsIndependentResultsUnknown(t *testing.T) {
	line := `{"type":"result","subtype":"success","is_error":false,"modelUsage":{"m":{"inputTokens":9,"outputTokens":2,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"costUSD":0.5}}}`
	_, usage := decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(line+"\n"+line+"\n"))
	if len(usage.Samples) != 2 || usage.Coverage != tracecontract.UsageCoverageUnknown {
		t.Fatalf("independent results must stay separate with unknown coverage: %+v", usage)
	}
	if usage.Samples[0].SessionID != "" || usage.Samples[0].ScopeID != "" {
		t.Errorf("missing identity must stay empty: %+v", usage.Samples[0])
	}
}

func TestHostUsageClaudeFailedResultZerosAreUnknownButSuccessZerosAreMeasured(t *testing.T) {
	failed := `{"type":"result","subtype":"error_during_execution","is_error":true,"session_id":"f","uuid":"u","modelUsage":{"m":{"inputTokens":0,"outputTokens":0,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"costUSD":0}}}`
	_, usage := decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(failed+"\n"))
	sample := usage.Samples[0]
	requireMetrics(t, sample, nil, nil, nil, nil)
	if sample.CostUSD != nil || sample.Finality != tracecontract.UsageFinalityPartial || usage.Coverage != tracecontract.UsageCoveragePartial {
		t.Errorf("failed result = %+v coverage=%q", sample, usage.Coverage)
	}
	success := `{"type":"result","subtype":"success","is_error":false,"session_id":"z","uuid":"u","modelUsage":{"m":{"inputTokens":0,"outputTokens":0,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"costUSD":0}}}`
	_, usage = decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(success+"\n"))
	sample = usage.Samples[0]
	requireMetrics(t, sample, i64(0), i64(0), i64(0), i64(0))
	if sample.CostUSD == nil || *sample.CostUSD != 0 {
		t.Errorf("successful zero cost must be measured: %+v", sample)
	}
}

func TestHostUsageClaudeWithoutResultIsUnknownAndUnknownEventsDowngrade(t *testing.T) {
	assistantOnly := `{"type":"assistant","session_id":"s","message":{"role":"assistant","usage":{"input_tokens":5,"output_tokens":7}}}`
	_, usage := decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(assistantOnly+"\n"))
	if len(usage.Samples) != 0 || usage.Coverage != tracecontract.UsageCoverageUnknown || !hasWarning(usage.Warnings, "usage_final_missing") {
		t.Errorf("assistant-only usage = %+v", usage)
	}
	result := `{"type":"result","subtype":"success","is_error":false,"session_id":"s","uuid":"u","modelUsage":{"m":{"inputTokens":1,"outputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}}`
	future := `{"type":"future_event","usage":{"input_tokens":5}}`
	_, usage = decodeHost(t, tracecontract.InputFormatClaudeJSON, []byte(result+"\n"+future+"\n"))
	if len(usage.Samples) != 1 || usage.Coverage != tracecontract.UsageCoveragePartial || !hasWarning(usage.Warnings, "usage_unknown_event") {
		t.Errorf("unknown event usage = %+v", usage)
	}
}

func TestHostUsageCodexIdenticalTurnsWithoutIdentityAreNotDeduplicated(t *testing.T) {
	_, usage := decodeHost(t, tracecontract.InputFormatCodexExec, fixture(t, "codex-turn-duplicate.jsonl"))
	if len(usage.Samples) != 2 || usage.Coverage != tracecontract.UsageCoverageUnknown {
		t.Fatalf("usage = %+v", usage)
	}
	for _, sample := range usage.Samples {
		requireMetrics(t, sample, i64(100), i64(10), i64(40), nil)
		if sample.Host != "codex" || sample.Temporality != tracecontract.UsageTemporalityDelta || sample.Finality != tracecontract.UsageFinalityFinal {
			t.Errorf("sample = %+v", sample)
		}
		if sample.SessionID != digest("session", "thr-codex-1") || sample.TurnID != "" || sample.CostUSD != nil || sample.CostBasis != tracecontract.UsageCostBasisUnknown {
			t.Errorf("identity/cost = %+v", sample)
		}
	}
}

func TestHostUsageCodexDeduplicatesOnlyWithStableTurnID(t *testing.T) {
	head := `{"type":"thread.started","thread_id":"thr"}` + "\n"
	same := `{"type":"turn.completed","turn_id":"t1","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":2}}` + "\n"
	other := `{"type":"turn.completed","turn_id":"t2","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":2}}` + "\n"
	conflict := `{"type":"turn.completed","turn_id":"t1","usage":{"input_tokens":99,"cached_input_tokens":0,"output_tokens":2}}` + "\n"

	_, usage := decodeHost(t, tracecontract.InputFormatCodexExec, []byte(head+same+same+other))
	if len(usage.Samples) != 2 || usage.Coverage != tracecontract.UsageCoverageComplete || !hasWarning(usage.Warnings, "usage_duplicate_event") {
		t.Errorf("duplicate handling = %+v", usage)
	}
	requireMetrics(t, usage.Samples[0], i64(10), i64(2), i64(0), nil)
	if usage.Samples[0].TurnID != digest("turn", "t1") {
		t.Errorf("turn digest = %q", usage.Samples[0].TurnID)
	}

	_, usage = decodeHost(t, tracecontract.InputFormatCodexExec, []byte(head+same+conflict))
	if len(usage.Samples) != 1 || usage.Coverage != tracecontract.UsageCoveragePartial || !hasWarning(usage.Warnings, "usage_conflicting_duplicate") {
		t.Errorf("conflicting duplicate = %+v", usage)
	}
	requireMetrics(t, usage.Samples[0], i64(10), i64(2), i64(0), nil)
}

func TestHostUsageCodexFailedAndUsagelessTurnsDowngradeCoverage(t *testing.T) {
	lines := `{"type":"thread.started","thread_id":"thr"}` + "\n" +
		`{"type":"turn.completed","turn_id":"t1","usage":{"input_tokens":1,"output_tokens":1}}` + "\n" +
		`{"type":"turn.failed","error":{"message":"SECRET-FAILURE"}}` + "\n" +
		`{"type":"turn.completed","turn_id":"t2"}` + "\n"
	_, usage := decodeHost(t, tracecontract.InputFormatCodexExec, []byte(lines))
	if len(usage.Samples) != 1 || usage.Coverage != tracecontract.UsageCoveragePartial {
		t.Fatalf("usage = %+v", usage)
	}
	if !hasWarning(usage.Warnings, "usage_turn_failed") || !hasWarning(usage.Warnings, "usage_missing") {
		t.Errorf("warnings = %q", usage.Warnings)
	}
	if strings.Contains(strings.Join(usage.Warnings, " "), "SECRET") {
		t.Errorf("warnings leaked payload: %q", usage.Warnings)
	}
}

func TestHostUsageOmoMessageEndIsFinalAuthority(t *testing.T) {
	// Given repeated message_update, turn_end and agent_end records for one assistant message
	input, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, fixture(t, "omo-final-authority.jsonl"))
	// Then the message is counted exactly once from message_end
	if len(usage.Samples) != 1 || input.Incomplete || usage.Coverage != tracecontract.UsageCoverageComplete {
		t.Fatalf("usage = %+v incomplete=%v", usage, input.Incomplete)
	}
	sample := usage.Samples[0]
	requireMetrics(t, sample, i64(120), i64(30), i64(0), i64(7))
	if sample.Host != "omo" || sample.Provider != "prov-a" || sample.Model != "model-a" || sample.Temporality != tracecontract.UsageTemporalityDelta || sample.Finality != tracecontract.UsageFinalityFinal {
		t.Errorf("sample = %+v", sample)
	}
	if sample.CostUSD == nil || *sample.CostUSD != 0.3 || sample.CostBasis != tracecontract.UsageCostBasisHostEstimate {
		t.Errorf("cost = %v basis=%q", sample.CostUSD, sample.CostBasis)
	}
	if sample.SessionID != digest("session", "sess-omo-1") || sample.MessageID != digest("message", "resp-omo-1") || sample.Epoch != "0" {
		t.Errorf("identity = %+v", sample)
	}
	if usage.Warnings == nil || len(usage.Warnings) != 0 {
		t.Errorf("warnings must be an empty non-nil slice: %#v", usage.Warnings)
	}
}

func TestHostUsageOmpCountsEachAssistantMessageEndOfAToolTurn(t *testing.T) {
	// Given a recorded-shape omp --mode json stream with a tool-use message and a final answer
	input, usage := decodeHost(t, tracecontract.InputFormatOmpJSON, fixture(t, "omp-tool-turn.jsonl"))
	// Then each assistant message_end is one omp delta sample and nothing else is counted
	if len(usage.Samples) != 2 || input.Incomplete || usage.Coverage != tracecontract.UsageCoverageComplete || len(usage.Warnings) != 0 {
		t.Fatalf("usage = %+v incomplete=%v", usage, input.Incomplete)
	}
	want := map[string]struct {
		input, output int64
		cost          float64
	}{
		digest("message", "resp-omp-1"): {100, 40, 0.0003},
		digest("message", "resp-omp-2"): {150, 20, 0.00025},
	}
	for _, sample := range usage.Samples {
		expected, ok := want[sample.MessageID]
		if !ok {
			t.Fatalf("unexpected sample %+v", sample)
		}
		delete(want, sample.MessageID)
		requireMetrics(t, sample, i64(expected.input), i64(expected.output), i64(0), i64(0))
		if sample.Host != "omp" || sample.Provider != "anthropic" || sample.Model != "claude-haiku-4-5" || sample.SessionID != digest("session", "sess-omp-1") ||
			sample.Finality != tracecontract.UsageFinalityFinal || sample.Temporality != tracecontract.UsageTemporalityDelta {
			t.Errorf("sample = %+v", sample)
		}
		if sample.CostUSD == nil || *sample.CostUSD != expected.cost || sample.CostBasis != tracecontract.UsageCostBasisHostEstimate {
			t.Errorf("cost = %v basis=%q", sample.CostUSD, sample.CostBasis)
		}
	}
	encoded, err := json.Marshal(usage)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "SECRET") || strings.Contains(string(encoded), "sess-omp-1") || strings.Contains(string(encoded), "resp-omp") {
		t.Errorf("usage leaked payload or raw identity: %s", encoded)
	}
}

func TestHostUsageOmoRepeatedMessageEndWithSameIdentityCountsOnce(t *testing.T) {
	line := omoEnd("resp-1", `{"input":4,"output":1,"cacheRead":0,"cacheWrite":0}`)
	_, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(line+"\n"+line+"\n"))
	if len(usage.Samples) != 1 || usage.Coverage != tracecontract.UsageCoverageComplete {
		t.Fatalf("usage = %+v", usage)
	}
	missing := `{"type":"message_end","message":{"role":"assistant","stopReason":"stop","usage":{"input":4,"output":1,"cacheRead":0,"cacheWrite":0}}}`
	_, usage = decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(missing+"\n"+missing+"\n"))
	if len(usage.Samples) != 2 || usage.Coverage != tracecontract.UsageCoverageUnknown {
		t.Fatalf("messages without identity must stay separate: %+v", usage)
	}
}

func TestHostUsageUnknownVersusMeasuredZero(t *testing.T) {
	_, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, fixture(t, "unknown-zero-redaction.jsonl"))
	if len(usage.Samples) != 2 || usage.Coverage != tracecontract.UsageCoveragePartial {
		t.Fatalf("usage = %+v", usage)
	}
	requireMetrics(t, usage.Samples[0], i64(5), i64(1), nil, nil)
	if usage.Samples[0].CostUSD != nil || usage.Samples[0].CostBasis != tracecontract.UsageCostBasisUnknown {
		t.Errorf("absent cost must be unknown: %+v", usage.Samples[0])
	}
	aborted := usage.Samples[1]
	requireMetrics(t, aborted, nil, nil, nil, nil)
	if aborted.CostUSD != nil || aborted.Finality != tracecontract.UsageFinalityPartial {
		t.Errorf("aborted zeros are unknown, not measured: %+v", aborted)
	}
}

func TestHostUsageMetricValidation(t *testing.T) {
	cases := []struct {
		name  string
		usage string
		want  *int64
		field func(tracecontract.UsageSample) *int64
	}{
		{"negative", `{"input":-1,"output":1}`, nil, func(s tracecontract.UsageSample) *int64 { return s.InputTokens }},
		{"fractional", `{"input":1.5,"output":1}`, nil, func(s tracecontract.UsageSample) *int64 { return s.InputTokens }},
		{"exponent", `{"input":1e3,"output":1}`, nil, func(s tracecontract.UsageSample) *int64 { return s.InputTokens }},
		{"string", `{"input":"12","output":1}`, nil, func(s tracecontract.UsageSample) *int64 { return s.InputTokens }},
		{"null", `{"input":null,"output":1}`, nil, func(s tracecontract.UsageSample) *int64 { return s.InputTokens }},
		{"overflow", `{"input":9223372036854775808,"output":1}`, nil, func(s tracecontract.UsageSample) *int64 { return s.InputTokens }},
		{"beyond float precision", `{"input":9007199254740993,"output":1}`, i64(9007199254740993), func(s tracecontract.UsageSample) *int64 { return s.InputTokens }},
		{"max int64", `{"input":9223372036854775807,"output":1}`, i64(9223372036854775807), func(s tracecontract.UsageSample) *int64 { return s.InputTokens }},
		{"sibling survives", `{"input":-1,"output":7}`, i64(7), func(s tracecontract.UsageSample) *int64 { return s.OutputTokens }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(omoEnd("m", tc.usage)+"\n"))
			if len(usage.Samples) != 1 || !samePtr(tc.field(usage.Samples[0]), tc.want) {
				t.Fatalf("usage = %+v want=%s", usage, deref(tc.want))
			}
			invalid := tc.name != "beyond float precision" && tc.name != "max int64" && tc.name != "null"
			if invalid && !hasWarning(usage.Warnings, "usage_invalid_metric") {
				t.Errorf("invalid metric must warn: %q", usage.Warnings)
			}
		})
	}
	negativeCost := omoEnd("c", `{"input":1,"output":1,"cost":{"total":-0.5}}`)
	_, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(negativeCost+"\n"))
	if usage.Samples[0].CostUSD != nil || !hasWarning(usage.Warnings, "usage_invalid_metric") {
		t.Errorf("negative cost = %+v", usage)
	}
	positiveCost := omoEnd("p", `{"input":1,"output":1,"cost":{"total":0.25}}`)
	_, usage = decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(positiveCost+"\n"))
	if usage.Samples[0].CostUSD == nil || *usage.Samples[0].CostUSD != 0.25 {
		t.Errorf("positive cost = %+v", usage.Samples[0])
	}
}

func TestHostUsageIdentityIsBoundedAndHashed(t *testing.T) {
	atLimit := strings.Repeat("a", 256)
	_, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(omoEnd(atLimit, `{"input":1,"output":1}`)+"\n"))
	if usage.Samples[0].MessageID != digest("message", atLimit) || usage.Coverage != tracecontract.UsageCoverageComplete {
		t.Errorf("256-byte identity = %+v", usage)
	}
	tooLong := strings.Repeat("a", 257)
	_, usage = decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(omoEnd(tooLong, `{"input":1,"output":1}`)+"\n"))
	if usage.Samples[0].MessageID != "" || usage.Coverage != tracecontract.UsageCoverageUnknown || !hasWarning(usage.Warnings, "usage_identity_too_long") {
		t.Errorf("257-byte identity = %+v", usage)
	}
}

func TestHostUsageDoesNotDiscloseRawPayloadOrOriginalIdentifiers(t *testing.T) {
	secrets := []string{"SECRET", "sess-claude-A", "thr-codex-1", "resp-omo-1", "sess-omo-1", "resp-aborted", "sk-", "res-2"}
	formats := map[string]string{
		"claude-cumulative-reset.jsonl": tracecontract.InputFormatClaudeJSON,
		"codex-turn-duplicate.jsonl":    tracecontract.InputFormatCodexExec,
		"omo-final-authority.jsonl":     tracecontract.InputFormatOmoJSON,
		"unknown-zero-redaction.jsonl":  tracecontract.InputFormatOmoJSON,
	}
	for name, format := range formats {
		input, usage := decodeHost(t, format, fixture(t, name))
		encoded, err := json.Marshal(usage)
		if err != nil {
			t.Fatal(err)
		}
		dump := string(encoded) + fmt.Sprintf("%+v", input)
		for _, secret := range secrets {
			if strings.Contains(dump, secret) {
				t.Errorf("%s leaked %q: %s", name, secret, dump)
			}
		}
	}
}

func TestHostDecodePartialFailuresKeepValidSamplesWithoutRawErrors(t *testing.T) {
	body := omoEnd("a", `{"input":1,"output":1}`) + "\n" + `{"type":"message_end" SECRET-RAW-LINE` + "\n" + omoEnd("b", `{"input":2,"output":2}`) + "\n"
	input, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(body))
	if len(usage.Samples) != 2 || !input.Incomplete || !hasWarning(input.Warnings, "invalid_jsonl_line") || usage.Coverage != tracecontract.UsageCoveragePartial {
		t.Fatalf("input=%+v usage=%+v", input, usage)
	}
	if strings.Contains(strings.Join(input.Warnings, " ")+strings.Join(usage.Warnings, " "), "SECRET-RAW-LINE") {
		t.Errorf("raw parser input leaked")
	}
	trailing := omoEnd("c", `{"input":1,"output":1}`) + ` {"extra":true}` + "\n"
	input, usage = decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(trailing))
	if len(usage.Samples) != 0 || !input.Incomplete {
		t.Errorf("trailing JSON on one line must be a malformed line: %+v %+v", input, usage)
	}
}

func TestHostDecodeSkipsOverLimitLineAndContinues(t *testing.T) {
	padding := func(total int) string {
		prefix := `{"type":"message_update","pad":"`
		return prefix + strings.Repeat("a", total-len(prefix)-2) + `"}`
	}
	good := func(id string) string { return omoEnd(id, `{"input":3,"output":1}`) }

	over := good("one") + "\n" + padding(tracecontract.HostLineMaxBytes+1) + "\n" + good("two") + "\n"
	input, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(over))
	if len(usage.Samples) != 2 || !input.Incomplete || !hasWarning(input.Warnings, "jsonl_line_too_long") || hasWarning(input.Warnings, "invalid_jsonl_line") {
		t.Fatalf("over-limit line: input=%+v samples=%d", input.Warnings, len(usage.Samples))
	}
	if usage.Coverage != tracecontract.UsageCoveragePartial {
		t.Errorf("coverage = %q", usage.Coverage)
	}

	exact := good("one") + "\n" + padding(tracecontract.HostLineMaxBytes) + "\n" + good("two") + "\n"
	input, usage = decodeHost(t, tracecontract.InputFormatOmoJSON, []byte(exact))
	if len(usage.Samples) != 2 || input.Incomplete || len(input.Warnings) != 0 {
		t.Errorf("a line of exactly the cap must be accepted: %+v", input)
	}
}

func TestDefaultFormatKeepsScannerCapAndOmitsUsage(t *testing.T) {
	long := `{"event":"step_end","step":"first","ok":false,"pad":"` + strings.Repeat("a", 70000) + `"}`
	body := []byte(`{"event":"step_end","step":"early","ok":false}` + "\n" + long + "\n" + `{"event":"step_end","step":"late","ok":false}` + "\n")
	for _, format := range []string{"", tracecontract.InputFormatIssueOps} {
		input := Source{}.Decode(body, format)
		if input.Usage != nil || !input.Incomplete || !hasWarning(input.Warnings, "jsonl_scan_error") || len(input.Lines) != 1 {
			t.Errorf("format %q: usage=%v incomplete=%v warnings=%q lines=%d", format, input.Usage, input.Incomplete, input.Warnings, len(input.Lines))
		}
	}
	input := Source{}.Decode(body, "no-such-format")
	if input.Usage != nil || !input.Incomplete || !hasWarning(input.Warnings, "unsupported_input_format") {
		t.Errorf("unsupported format = %+v", input)
	}
}

func TestHostLoadRequiresExplicitInputAndNeverFallsBackToState(t *testing.T) {
	calls := 0
	source := Source{ReadState: func(string) (statecontract.StateResult, error) {
		calls++
		return statecontract.StateResult{}, nil
	}}
	for _, format := range []string{tracecontract.InputFormatClaudeJSON, tracecontract.InputFormatCodexExec, tracecontract.InputFormatOmoJSON} {
		_, _, _, err := source.Load(filepath.Join(t.TempDir(), "missing.jsonl"), format)
		if err == nil {
			t.Errorf("%s: missing file must fail", format)
		}
	}
	if calls != 0 {
		t.Fatalf("host formats must not read state keys, ReadState calls=%d", calls)
	}
	if _, _, _, err := source.Load(filepath.Join(t.TempDir(), "missing.jsonl"), tracecontract.InputFormatIssueOps); err != nil || calls != 1 {
		t.Errorf("default mode keeps the state-key fallback: err=%v calls=%d", err, calls)
	}
}

func TestHostLoadReadsStdinAndFiles(t *testing.T) {
	line := omoEnd("x", `{"input":1,"output":1}`) + "\n"
	source := Source{Stdin: strings.NewReader(line)}
	name, body, truncated, err := source.Load("-", tracecontract.InputFormatOmoJSON)
	if err != nil || name != "stdin" || truncated || string(body) != line {
		t.Fatalf("stdin load = %q %q %v %v", name, body, truncated, err)
	}
	path := filepath.Join(t.TempDir(), "in.jsonl")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	name, body, truncated, err = Source{}.Load(path, tracecontract.InputFormatOmoJSON)
	if err != nil || name != "file" || truncated || string(body) != line {
		t.Fatalf("file load = %q %q %v %v", name, body, truncated, err)
	}
}

func TestHostLoadTruncatesAtFileCapAtLineBoundary(t *testing.T) {
	line := omoEnd("x", `{"input":1,"output":1}`) + "\n"
	path := filepath.Join(t.TempDir(), "big.jsonl")
	payload := append([]byte(line), bytes.Repeat([]byte("a"), tracecontract.HostInputMaxBytes)...)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	_, body, truncated, err := Source{}.Load(path, tracecontract.InputFormatOmoJSON)
	if err != nil || !truncated || string(body) != line {
		t.Fatalf("truncated=%v err=%v bodyLen=%d", truncated, err, len(body))
	}
	_, usage := decodeHost(t, tracecontract.InputFormatOmoJSON, body)
	if len(usage.Samples) != 1 {
		t.Errorf("complete lines before the cap must survive: %+v", usage)
	}

	exact := filepath.Join(t.TempDir(), "exact.jsonl")
	padding := tracecontract.HostInputMaxBytes - len(line)
	exactPayload := append([]byte(line), bytes.Repeat([]byte("a"), padding-1)...)
	exactPayload = append(exactPayload, '\n')
	if err := os.WriteFile(exact, exactPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	_, body, truncated, err = Source{}.Load(exact, tracecontract.InputFormatOmoJSON)
	if err != nil || truncated || len(body) != tracecontract.HostInputMaxBytes {
		t.Errorf("a file of exactly the cap is complete: truncated=%v err=%v len=%d", truncated, err, len(body))
	}
}
