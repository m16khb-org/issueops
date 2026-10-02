package trace

import (
	"errors"
	tracedomain "issueops/internal/domain/trace"
	"testing"

	tracecontract "issueops/internal/contract/trace"
)

type fakeEffects struct {
	loaded      bool
	analyzed    bool
	loadErr     error
	facts       *tracedomain.Input
	truncated   bool
	body        string
	loadFormat  string
	decodeCalls int
}

func (f *fakeEffects) Load(_ string, format string) (string, []byte, bool, error) {
	f.loaded = true
	f.loadFormat = format
	body := "{}"
	if f.body != "" {
		body = f.body
	}
	return "state", []byte(body), f.truncated, f.loadErr
}
func (f *fakeEffects) Decode([]byte, string) tracedomain.Input {
	f.analyzed = true
	f.decodeCalls++
	if f.facts != nil {
		return *f.facts
	}
	return tracedomain.Input{Document: &tracedomain.Document{Summary: tracedomain.Summary{FailedSteps: 1, FailureClass: "failure"}}}
}

func TestAnalyzeRejectsMissingInputBeforeEffects(t *testing.T) {
	f := &fakeEffects{}
	result, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: " "})
	if err == nil || result.OK || f.loaded || f.analyzed {
		t.Fatalf("result=%+v effects=%+v err=%v", result, f, err)
	}
}

func TestAnalyzePreservesSourceAndCount(t *testing.T) {
	f := &fakeEffects{}
	result, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: " key "})
	if err != nil || !result.OK || result.Input != "key" || result.InputSource != "state" || result.FindingCount != 1 || !f.analyzed {
		t.Fatalf("result=%+v effects=%+v err=%v", result, f, err)
	}
}

func TestAnalyzeReportsCompletenessIndependentlyOfExecution(t *testing.T) {
	for _, incomplete := range []bool{false, true} {
		// Given
		facts := tracedomain.Input{Incomplete: incomplete, Lines: []tracedomain.Document{{Event: "step_end", Step: "sentinel", OK: false}}}
		if incomplete {
			facts.Warnings = []string{"jsonl_scan_error"}
		}
		f := &fakeEffects{facts: &facts}
		// When
		result, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: "key"})
		// Then
		if err != nil || !result.OK || result.Complete == incomplete || result.FindingCount != 1 {
			t.Errorf("result=%+v err=%v", result, err)
		}
		if incomplete && (len(result.Warnings) != 1 || result.Warnings[0] != "jsonl_scan_error") {
			t.Errorf("warnings=%q", result.Warnings)
		}
	}
}

func TestAnalyzeDefaultsAndValidatesInputFormat(t *testing.T) {
	f := &fakeEffects{}
	if _, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: "key"}); err != nil || f.loadFormat != tracecontract.InputFormatIssueOps {
		t.Errorf("default format = %q err=%v", f.loadFormat, err)
	}
	for _, format := range []string{tracecontract.InputFormatClaudeJSON, tracecontract.InputFormatCodexExec, tracecontract.InputFormatOmoJSON} {
		f := &fakeEffects{}
		if _, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: "key", InputFormat: " " + format + " "}); err != nil || f.loadFormat != format {
			t.Errorf("format %q forwarded as %q err=%v", format, f.loadFormat, err)
		}
	}
	f = &fakeEffects{}
	result, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: "key", InputFormat: "jsonl-ish"})
	if err == nil || result.OK || result.Complete || f.loaded || len(result.Warnings) != 1 || result.Warnings[0] != "unsupported_input_format" {
		t.Fatalf("unsupported format: result=%+v effects=%+v err=%v", result, f, err)
	}
}

func TestAnalyzeCarriesUsageAndTruncation(t *testing.T) {
	one := int64(1)
	report := &tracecontract.UsageReport{Coverage: tracecontract.UsageCoverageComplete, Warnings: []string{}, Samples: []tracecontract.UsageSample{{Host: "omo", InputTokens: &one}}}
	facts := tracedomain.Input{Usage: report}
	f := &fakeEffects{facts: &facts}
	result, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: "x.jsonl", InputFormat: tracecontract.InputFormatOmoJSON})
	if err != nil || !result.OK || !result.Complete || result.Usage != report || len(result.Warnings) != 0 {
		t.Fatalf("usage passthrough: result=%+v err=%v", result, err)
	}

	facts = tracedomain.Input{Usage: &tracecontract.UsageReport{Coverage: tracecontract.UsageCoverageComplete, Warnings: []string{}, Samples: []tracecontract.UsageSample{{Host: "omo", InputTokens: &one}}}}
	f = &fakeEffects{facts: &facts, truncated: true}
	result, err = (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: "x.jsonl", InputFormat: tracecontract.InputFormatOmoJSON})
	if err != nil || !result.OK || result.Complete || result.Usage.Coverage != tracecontract.UsageCoveragePartial {
		t.Fatalf("truncation: result=%+v err=%v", result, err)
	}
	for _, want := range []string{"input_truncated"} {
		found := false
		for _, warning := range result.Warnings {
			found = found || warning == want
		}
		if !found {
			t.Errorf("warnings %q missing %q", result.Warnings, want)
		}
	}

	onlyTruncatedFragment := &fakeEffects{body: " ", truncated: true, facts: &tracedomain.Input{Usage: &tracecontract.UsageReport{Coverage: tracecontract.UsageCoverageUnknown, Warnings: []string{}, Samples: []tracecontract.UsageSample{}}}}
	result, err = (Service{Effects: onlyTruncatedFragment}).Analyze(tracecontract.TraceAnalyzeRequest{Input: "x.jsonl", InputFormat: tracecontract.InputFormatOmoJSON})
	if err != nil || result.Complete || onlyTruncatedFragment.decodeCalls != 1 {
		t.Errorf("truncated empty body must report incompleteness, not an empty-input error: result=%+v err=%v", result, err)
	}
}

func TestAnalyzeReportsReadFailureWithoutDecodingPartialBody(t *testing.T) {
	// Given
	readErr := errors.New("fixture read failure")
	f := &fakeEffects{loadErr: readErr}
	// When
	result, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: "key"})
	// Then
	if !errors.Is(err, readErr) || result.OK || result.Complete || !f.loaded || f.analyzed {
		t.Fatalf("result=%+v effects=%+v err=%v", result, f, err)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "trace_read_error" {
		t.Errorf("warnings=%q", result.Warnings)
	}
}
