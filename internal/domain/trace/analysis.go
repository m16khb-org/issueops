package trace

import (
	"fmt"
	"issueops/internal/domain/policy"
	classification "issueops/internal/domain/traceclassification"
)

func Analyze(input Input) Analysis {
	findings := []Finding{}
	types := []string{}
	warnings := append([]string{}, input.Warnings...)
	if input.Document != nil {
		doc := *input.Document
		findings = append(findings, selfVerifySummaryFindings(doc.Summary)...)
		findings = append(findings, guardFindings(doc.GuardRules)...)
		if doc.Upkeep.Kind != "" || doc.Upkeep.Summary != "" {
			findings = append(findings, docUpkeepFindings([]Upkeep{doc.Upkeep})...)
		}
		if len(findings) > 0 {
			types = append(types, traceTypesForJSON(doc)...)
		}
	} else {
		findings, types = analyzeLines(input.Lines)
	}
	if len(findings) == 0 && input.Usage == nil {
		warnings = append(warnings, "no_supported_trace_findings")
	}
	return Analysis{Findings: dedupeTraceFindings(findings), Types: uniqSortedTraceStrings(types), Warnings: warnings, Incomplete: input.Incomplete}
}
func analyzeLines(lines []Document) ([]Finding, []string) {
	events := []Upkeep{}
	failedSteps := map[string]int{}
	types := []string{}
	for _, doc := range lines {
		if doc.Upkeep.Kind != "" || doc.Upkeep.Summary != "" {
			events = append(events, doc.Upkeep)
			types = append(types, "doc_upkeep_jsonl")
			continue
		}
		if doc.Event == "step_end" && !doc.OK {
			step := doc.Step
			if step == "" {
				step = "unknown step"
			}
			failedSteps[step]++
			types = append(types, "self_verify_progress_jsonl")
		}
	}
	findings := []Finding{}
	findings = append(findings, docUpkeepFindings(events)...)
	for _, step := range traceSortedIntKeys(failedSteps) {
		findings = append(findings, Finding{FailureClass: "self_verify_progress_failure", RecurringPattern: fmt.Sprintf("%s failed %d time(s)", policy.RedactFreeform(step), failedSteps[step]), ProposedKnob: classification.ProposedKnobForStep(step), OverfitRisk: "medium: progress JSONL may capture one run; rerun before changing harness behavior", VerificationCommand: classification.DefaultVerificationCommand(step)})
	}
	return findings, types
}
