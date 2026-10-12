package trace

import (
	classification "issueops/internal/domain/traceclassification"
	"slices"
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

func TestAnalyzeDocumentTraceTypes(t *testing.T) {
	failing := Summary{FailedSteps: 1}
	for name, tc := range map[string]struct {
		doc      Document
		findings int
		types    []string
	}{
		"guard flag without rules has no findings": {Document{HasGuard: true}, 0, nil},
		"summary flag without failures":            {Document{HasSummary: true}, 0, nil},
		"guard rules without flag":                 {Document{GuardRules: []string{"r"}}, 1, nil},
		"guard rules with flag":                    {Document{HasGuard: true, GuardRules: []string{"r"}}, 1, []string{"guard_result"}},
		"summary flag":                             {Document{HasSummary: true, Summary: failing}, 1, []string{"self_verify_summary"}},
		"top failure class":                        {Document{TopFailureClass: "x", Summary: failing}, 1, []string{"self_verify_summary"}},
		"top failed steps":                         {Document{TopFailedSteps: 1, Summary: failing}, 1, []string{"self_verify_summary"}},
		"zero top failed steps":                    {Document{TopFailedSteps: 0, Summary: failing}, 1, nil},
		"upkeep kind":                              {Document{Upkeep: Upkeep{Kind: "k"}}, 1, []string{"doc_upkeep_json"}},
		"upkeep summary":                           {Document{Upkeep: Upkeep{Summary: "s"}}, 1, []string{"doc_upkeep_json"}},
		"empty document":                           {Document{}, 0, nil},
	} {
		result := Analyze(Input{Document: &tc.doc})
		if len(result.Findings) != tc.findings || !slices.Equal(result.Types, tc.types) {
			t.Errorf("%s: findings=%d types=%q", name, len(result.Findings), result.Types)
		}
	}
}

func TestAnalyzeSummaryFindingDefaults(t *testing.T) {
	const defaultPattern = "self-verification reported failed steps"
	for name, tc := range map[string]struct {
		summary        Summary
		class, pattern string
	}{
		"failed steps only":  {Summary{FailedSteps: 2}, "self_verification_failure", defaultPattern},
		"failed step only":   {Summary{FailedStep: "policy check"}, "self_verification_failure", "policy check"},
		"failure class only": {Summary{FailureClass: "flaky"}, "flaky", defaultPattern},
		"clusters override": {Summary{FailedSteps: 3, FailedStep: "a", Clusters: []Cluster{{Step: "a", Count: 2}, {Step: "", Count: 9}, {Step: "b", Count: 1}}},
			"self_verification_failure", "a failed 2 time(s); b failed 1 time(s)"},
		"clusters without step keep failed step": {Summary{FailedStep: "a", Clusters: []Cluster{{Count: 4}}}, "self_verification_failure", "a"},
	} {
		result := Analyze(Input{Document: &Document{Summary: tc.summary}})
		if len(result.Findings) != 1 || result.Findings[0].FailureClass != tc.class || result.Findings[0].RecurringPattern != tc.pattern {
			t.Errorf("%s: %+v", name, result.Findings)
		}
	}
	none := Analyze(Input{Document: &Document{}})
	if len(none.Findings) != 0 || !slices.Contains(none.Warnings, "no_supported_trace_findings") {
		t.Errorf("zero summary must yield nothing: %+v", none)
	}
	plain := Analyze(Input{Document: &Document{Summary: Summary{FailedStep: "policy check"}}}).Findings[0]
	if plain.VerificationCommand != classification.DefaultVerificationCommand("policy check") {
		t.Errorf("default verification command = %q", plain.VerificationCommand)
	}
}

func TestAnalyzeGuardFindingsNameEmptyRuleAndCount(t *testing.T) {
	result := Analyze(Input{Document: &Document{HasGuard: true, GuardRules: []string{"", "b", "", "a"}}})
	got := []string{}
	for _, finding := range result.Findings {
		got = append(got, finding.FailureClass+"|"+finding.RecurringPattern)
	}
	want := []string{"guard_a|a reported 1 time(s)", "guard_b|b reported 1 time(s)", "guard_guard_finding|guard_finding reported 2 time(s)"}
	if !slices.Equal(got, want) {
		t.Errorf("guard findings = %q", got)
	}
}

func TestAnalyzeUpkeepTargetsAreGroupedAndCounted(t *testing.T) {
	result := Analyze(Input{Lines: []Document{
		{Upkeep: Upkeep{Kind: "changed", TargetDocs: []string{"ADR.md", "TESTING.md"}}},
		{Upkeep: Upkeep{Kind: "other", TargetDocs: []string{"ADR.md", "TESTING.md"}}},
		{Upkeep: Upkeep{Kind: "kindonly"}},
		{Upkeep: Upkeep{Summary: "no target"}},
	}})
	got := []string{}
	for _, finding := range result.Findings {
		got = append(got, finding.RecurringPattern)
	}
	slices.Sort(got)
	want := []string{"ADR.md,TESTING.md queued 2 time(s)", "doc_upkeep queued 1 time(s)", "kindonly queued 1 time(s)"}
	if !slices.Equal(got, want) {
		t.Errorf("upkeep patterns = %q", got)
	}
}
