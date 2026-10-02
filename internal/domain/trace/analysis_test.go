package trace

import (
	"strings"
	"testing"
)

func TestAnalyzeKeepsUpkeepPriorityAndCountsFailedProgress(t *testing.T) {
	result := Analyze(Input{Lines: []Document{
		{Event: "step_end", OK: false, Step: "ignored", Upkeep: Upkeep{Kind: "changed", TargetDocs: []string{"ADR.md"}}},
		{Event: "step_end", OK: false, Step: "policy check"},
		{Event: "step_end", OK: false, Step: "policy check"},
		{Event: "step_end", OK: true, Step: "successful"},
	}})
	if result.Incomplete || len(result.Findings) != 2 || len(result.Types) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected fallback=%+v", result)
	}
	if result.Findings[0].FailureClass != "lifecycle_doc_upkeep" || result.Findings[1].RecurringPattern != "policy check failed 2 time(s)" {
		t.Fatalf("priority/count changed: %+v", result.Findings)
	}
	empty := Analyze(Input{Incomplete: true, Warnings: []string{"invalid_json:invalid_jsonl_line"}})
	if !empty.Incomplete || len(empty.Findings) != 0 || len(empty.Warnings) != 2 || empty.Warnings[0] != "invalid_json:invalid_jsonl_line" || empty.Warnings[1] != "no_supported_trace_findings" {
		t.Fatalf("invalid input lost: %+v", empty)
	}
}

func TestAnalyzeKeepsExplicitRerunAndRedactsSummary(t *testing.T) {
	result := Analyze(Input{Document: &Document{HasSummary: true, Summary: Summary{FailedSteps: 1, FailureClass: "deterministic", FailedStep: "token=hidden-value", Clusters: []Cluster{{Step: "token=hidden-value", Count: 2}}, RerunCommands: []string{"custom --token=hidden-value"}, Cause: "unrecognized", Evidence: []Evidence{{Cause: "unsupported", Code: "token=hidden-value", Source: "token=hidden-value"}}}}})
	if len(result.Findings) != 1 {
		t.Fatal(result)
	}
	finding := result.Findings[0]
	if finding.FailureCause != "unknown" || len(finding.FailureCauseEvidence) != 1 || finding.FailureCauseEvidence[0].Cause != "unknown" {
		t.Fatal(finding)
	}
	if finding.VerificationCommand != "<redacted>" {
		t.Fatalf("explicit rerun replaced: %+v", finding)
	}
	explicit := Analyze(Input{Document: &Document{Summary: Summary{FailedSteps: 1, RerunCommands: []string{"custom-check --all"}}}})
	if explicit.Findings[0].VerificationCommand != "custom-check --all" {
		t.Fatalf("explicit command replaced: %+v", explicit)
	}
	text := finding.RecurringPattern + finding.VerificationCommand + finding.FailureCauseEvidence[0].Code + finding.FailureCauseEvidence[0].Source
	if strings.Contains(text, "hidden-value") {
		t.Fatalf("secret leaked: %+v", finding)
	}
}

func TestAnalyzePreservesIncompleteWarningsWithOrWithoutFindings(t *testing.T) {
	for _, lines := range [][]Document{
		nil,
		{{Event: "step_end", Step: "sentinel", OK: false}},
	} {
		// Given
		input := Input{Lines: lines, Incomplete: true, Warnings: []string{"invalid_jsonl_line", "jsonl_scan_error"}}
		// When
		result := Analyze(input)
		// Then
		if !result.Incomplete || len(result.Findings) != len(lines) {
			t.Errorf("incomplete=%v findings=%d want %d", result.Incomplete, len(result.Findings), len(lines))
		}
		wantWarnings := 2
		if len(lines) == 0 {
			wantWarnings++
		}
		if len(result.Warnings) != wantWarnings || result.Warnings[0] != "invalid_jsonl_line" || result.Warnings[1] != "jsonl_scan_error" {
			t.Errorf("warnings=%q", result.Warnings)
		}
	}
}
