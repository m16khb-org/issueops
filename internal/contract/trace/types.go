// Package trace 패키지는 trace capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package trace

import failurecause "issueops/internal/contract/failurecause"

// AnalyzeRequest 타입은 trace 분석 입력이다. InputFormat이 비어 있으면 issueops 형식으로 본다.
type AnalyzeRequest struct {
	Input       string `json:"input"`
	InputFormat string `json:"input_format,omitempty"`
}

// AnalyzeResult 타입은 trace 분석 결과다. Complete가 false이면 분석이 불완전하다는 뜻이다.
// Usage는 host가 보고한 사용량이 있을 때만 채운다.
type AnalyzeResult struct {
	OK           bool              `json:"ok"`
	Complete     bool              `json:"complete"`
	Kind         string            `json:"kind"`
	Input        string            `json:"input"`
	InputSource  string            `json:"input_source"`
	TraceTypes   []string          `json:"trace_types"`
	FindingCount int               `json:"finding_count"`
	Findings     []AnalysisFinding `json:"findings"`
	Warnings     []string          `json:"warnings,omitempty"`
	Usage        *UsageReport      `json:"usage,omitempty"`
}

// AnalysisFinding 타입은 trace에서 찾은 실패 하나다. 분류와 원인 증거, 반복 패턴, 제안하는 조정값(knob), 과적합 위험, 검증 명령을 담는다.
type AnalysisFinding struct {
	FailureClass         string                  `json:"failure_class"`
	FailureCause         failurecause.Cause      `json:"failure_cause"`
	FailureCauseEvidence []failurecause.Evidence `json:"failure_cause_evidence"`
	RecurringPattern     string                  `json:"recurring_pattern"`
	ProposedKnob         string                  `json:"proposed_knob"`
	OverfitRisk          string                  `json:"overfit_risk"`
	VerificationCommand  string                  `json:"verification_command"`
}
