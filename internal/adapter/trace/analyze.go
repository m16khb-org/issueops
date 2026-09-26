package trace

import (
	traceapp "issueops/internal/application/trace"
	tracecontract "issueops/internal/contract/trace"
)

const TraceAnalysisKind = traceapp.AnalysisKind

func TraceAnalyze(req tracecontract.TraceAnalyzeRequest) (tracecontract.TraceAnalyzeResult, error) {
	return (traceapp.Service{Effects: traceEffects{}}).Analyze(req)
}

type traceEffects struct{}

func (traceEffects) Load(input string) (string, []byte, error) {
	loaded, err := loadTraceAnalysisInput(input)
	return loaded.Source, loaded.Body, err
}
func (traceEffects) Analyze(body []byte) ([]tracecontract.TraceAnalysisFinding, []string, []string) {
	return analyzeTraceBytes(body)
}
