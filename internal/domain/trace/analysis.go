// Package trace 패키지는 self-verify 요약, guard 규칙, 문서 관리(upkeep) 이벤트,
// 호스트 사용량 관측을 입력으로 받아 반복 실패 패턴 finding을 만드는 순수 domain
// 규칙이다. W3C traceparent 파싱도 여기서 맡고, 입력 수집과 출력은 어댑터가 담당한다.
package trace

import (
	"fmt"
	"issueops/internal/domain/policy"
	"issueops/internal/domain/traceclassification"
)

// Analyze 함수는 input의 구조화 Document(없으면 Lines의 JSONL 줄들)에서 실패
// 분류, 원인, 반복 패턴, 제안 knob이 담긴 Finding을 만들어 중복을 제거해 돌려준다.
// 지원하는 finding이 없고 Usage도 없으면 no_supported_trace_findings 경고를 붙인다.
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
		findings = append(findings, Finding{FailureClass: "self_verify_progress_failure", RecurringPattern: fmt.Sprintf("%s failed %d time(s)", policy.RedactFreeform(step), failedSteps[step]), ProposedKnob: traceclassification.ProposedKnobForStep(step), OverfitRisk: "medium: progress JSONL may capture one run; rerun before changing harness behavior", VerificationCommand: traceclassification.DefaultVerificationCommand(step)})
	}
	return findings, types
}
