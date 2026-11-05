// Package trace is the application service that analyzes agent execution
// traces and reports findings with failure causes.
package trace

import (
	"fmt"
	failurecontract "issueops/internal/contract/failurecause"
	failurecausedomain "issueops/internal/domain/failurecause"
	tracedomain "issueops/internal/domain/trace"
	"strings"

	tracecontract "issueops/internal/contract/trace"
)

const analysisKind = "trace_analysis"

type effects interface {
	Load(input, format string) (source string, body []byte, truncated bool, err error)
	Decode(body []byte, format string) tracedomain.Input
}

// Service analyzes agent execution traces. Effects supplies trace loading and decoding.
type Service struct{ Effects effects }

// Analyze loads the trace named by req.Input, decodes it in req.InputFormat
// (default issueops), and returns the classified findings. It returns an error
// for empty input, an unsupported format, or an unreadable trace; the partial
// result then carries warnings.
func (service Service) Analyze(req tracecontract.AnalyzeRequest) (tracecontract.AnalyzeResult, error) {
	input := strings.TrimSpace(req.Input)
	result := tracecontract.AnalyzeResult{
		OK: false, Kind: analysisKind, Input: input,
		TraceTypes: []string{}, Findings: []tracecontract.AnalysisFinding{}, Warnings: []string{},
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
		return result, fmt.Errorf("unsupported trace input format %q (want issueops, claude-stream, codex-exec, omo-json, omp-json or opencode-json)", format)
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
	classifyDocument(inputFacts.Document)
	analysis := tracedomain.Analyze(inputFacts)
	result.TraceTypes = analysis.Types
	result.Findings = toContractFindings(analysis.Findings)
	result.FindingCount = len(result.Findings)
	result.Warnings = analysis.Warnings
	result.Complete = !analysis.Incomplete
	result.Usage = inputFacts.Usage
	result.OK = true
	return result, nil
}

func supportedInputFormat(format string) bool {
	switch format {
	case tracecontract.InputFormatIssueOps, tracecontract.InputFormatClaudeJSON, tracecontract.InputFormatCodexExec, tracecontract.InputFormatOmoJSON, tracecontract.InputFormatOmpJSON, tracecontract.InputFormatOpencodeJSON:
		return true
	}
	return false
}

// classifyDocument rewrites the document summary cause and evidence through
// the failure-cause classifier. A nil document is left alone.
func classifyDocument(document *tracedomain.Document) {
	if document == nil {
		return
	}
	summary := &document.Summary
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

func toContractFindings(findings []tracedomain.Finding) []tracecontract.AnalysisFinding {
	out := make([]tracecontract.AnalysisFinding, 0, len(findings))
	for _, f := range findings {
		finding := tracecontract.AnalysisFinding{FailureClass: f.FailureClass, FailureCause: failurecontract.Cause(f.FailureCause), FailureCauseEvidence: make([]failurecontract.Evidence, 0, len(f.FailureCauseEvidence)), RecurringPattern: f.RecurringPattern, ProposedKnob: f.ProposedKnob, OverfitRisk: f.OverfitRisk, VerificationCommand: f.VerificationCommand}
		for _, e := range f.FailureCauseEvidence {
			finding.FailureCauseEvidence = append(finding.FailureCauseEvidence, failurecontract.Evidence{Cause: failurecontract.Cause(e.Cause), Code: e.Code, Source: e.Source})
		}
		out = append(out, finding)
	}
	return out
}
