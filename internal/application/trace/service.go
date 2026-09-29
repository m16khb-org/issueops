package trace

import (
	"fmt"
	failurecontract "issueops/internal/contract/failurecause"
	failurecausedomain "issueops/internal/domain/failurecause"
	tracedomain "issueops/internal/domain/trace"
	"strings"

	tracecontract "issueops/internal/contract/trace"
)

const AnalysisKind = "trace_analysis"

type Effects interface {
	Load(string) (source string, body []byte, err error)
	Decode([]byte) tracedomain.Input
}

type Service struct{ Effects Effects }

func (service Service) Analyze(req tracecontract.TraceAnalyzeRequest) (tracecontract.TraceAnalyzeResult, error) {
	input := strings.TrimSpace(req.Input)
	result := tracecontract.TraceAnalyzeResult{
		OK: false, Kind: AnalysisKind, Input: input,
		TraceTypes: []string{}, Findings: []tracecontract.TraceAnalysisFinding{}, Warnings: []string{},
	}
	if input == "" {
		return result, fmt.Errorf("trace analyze input is required")
	}
	source, body, err := service.Effects.Load(input)
	if err != nil {
		return result, err
	}
	result.InputSource = source
	if len(strings.TrimSpace(string(body))) == 0 {
		return result, fmt.Errorf("trace analyze input is empty")
	}
	inputFacts := service.Effects.Decode(body)
	if inputFacts.Document != nil {
		summary := &inputFacts.Document.Summary
		evidence := make([]failurecontract.Evidence, 0, len(summary.Evidence))
		for _, e := range tracedomain.NormalizeEvidence(summary.Evidence) {
			evidence = append(evidence, failurecontract.Evidence{Cause: failurecontract.Cause(e.Cause), Code: e.Code, Source: e.Source})
		}
		classified := failurecausedomain.Classify(true, evidence)
		summary.Cause = string(classified.Cause)
		summary.Evidence = make([]tracedomain.Evidence, 0, len(classified.Evidence))
		for _, e := range classified.Evidence {
			summary.Evidence = append(summary.Evidence, tracedomain.Evidence{Cause: string(e.Cause), Code: e.Code, Source: e.Source})
		}
	}
	analysis := tracedomain.Analyze(inputFacts)
	result.TraceTypes = analysis.Types
	result.Findings = make([]tracecontract.TraceAnalysisFinding, 0, len(analysis.Findings))
	for _, f := range analysis.Findings {
		finding := tracecontract.TraceAnalysisFinding{FailureClass: f.FailureClass, FailureCause: failurecontract.Cause(f.FailureCause), FailureCauseEvidence: make([]failurecontract.Evidence, 0, len(f.FailureCauseEvidence)), RecurringPattern: f.RecurringPattern, ProposedKnob: f.ProposedKnob, OverfitRisk: f.OverfitRisk, VerificationCommand: f.VerificationCommand}
		for _, e := range f.FailureCauseEvidence {
			finding.FailureCauseEvidence = append(finding.FailureCauseEvidence, failurecontract.Evidence{Cause: failurecontract.Cause(e.Cause), Code: e.Code, Source: e.Source})
		}
		result.Findings = append(result.Findings, finding)
	}
	result.FindingCount = len(result.Findings)
	result.Warnings = analysis.Warnings
	result.OK = true
	return result, nil
}
