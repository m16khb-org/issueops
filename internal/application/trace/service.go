package trace

import (
	"fmt"
	"strings"

	tracecontract "issueops/internal/contract/trace"
)

const AnalysisKind = "trace_analysis"

type Effects interface {
	Load(string) (source string, body []byte, err error)
	Analyze([]byte) ([]tracecontract.TraceAnalysisFinding, []string, []string)
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
	findings, traceTypes, warnings := service.Effects.Analyze(body)
	result.TraceTypes = traceTypes
	result.Findings = findings
	result.FindingCount = len(findings)
	result.Warnings = warnings
	result.OK = true
	return result, nil
}
