package trace

import (
	"strings"
	"testing"
)

func TestAnalyzeKeepsUpkeepPriorityAndCountsFailedProgress(t *testing.T) {
	result := Analyze(Input{JSONError: "multiple documents", Lines: []Document{
		{Event: "step_end", OK: false, Step: "ignored", Upkeep: Upkeep{Kind: "changed", TargetDocs: []string{"ADR.md"}}},
		{Event: "step_end", OK: false, Step: "policy check"},
		{Event: "step_end", OK: false, Step: "policy check"},
		{Event: "step_end", OK: true, Step: "successful"},
	}})
	if len(result.Findings) != 2 || len(result.Types) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected fallback=%+v", result)
	}
	if result.Findings[0].FailureClass != "lifecycle_doc_upkeep" || result.Findings[1].RecurringPattern != "policy check failed 2 time(s)" {
		t.Fatalf("priority/count changed: %+v", result.Findings)
	}
	empty := Analyze(Input{JSONError: "broken"})
	if len(empty.Findings) != 0 || len(empty.Warnings) != 1 || empty.Warnings[0] != "invalid_json:broken" {
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
