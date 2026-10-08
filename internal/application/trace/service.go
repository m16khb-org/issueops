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
	Load(input, format string) (source string, body []byte, truncated bool, err error)
	Decode(body []byte, format string) tracedomain.Input
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
	format := strings.TrimSpace(req.InputFormat)
	if format == "" {
		format = tracecontract.InputFormatIssueOps
	}
	if !supportedInputFormat(format) {
		result.Warnings = append(result.Warnings, "unsupported_input_format")
		return result, fmt.Errorf("unsupported trace input format %q (want issueops, claude-stream, codex-exec, omo-json or omp-json)", format)
	}
	source, body, truncated, err := service.Effects.Load(input, format)
	result.InputSource = source
	if err != nil {
		result.Warnings = append(result.Warnings, "trace_read_error")
		return result, fmt.Errorf("read trace input: %w", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 && !truncated {
		return result, fmt.Errorf("trace analyze input is empty")
	}
	inputFacts := service.Effects.Decode(body, format)
	if truncated {
		inputFacts.Incomplete = true
		inputFacts.Warnings = append(inputFacts.Warnings, "input_truncated")
		if inputFacts.Usage != nil {
			tracedomain.NoteUsageLoss(inputFacts.Usage, "input_truncated")
		}
	}
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
	result.Complete = !analysis.Incomplete
	result.Usage = inputFacts.Usage
	result.OK = true
	return result, nil
}

func supportedInputFormat(format string) bool {
	switch format {
	case tracecontract.InputFormatIssueOps, tracecontract.InputFormatClaudeJSON, tracecontract.InputFormatCodexExec, tracecontract.InputFormatOmoJSON, tracecontract.InputFormatOmpJSON:
		return true
	}
	return false
}
