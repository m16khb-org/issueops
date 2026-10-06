package trace

import (
	statestore "issueops/internal/adapter/outbound/state"
	traceapp "issueops/internal/application/trace"
	tracecontract "issueops/internal/contract/trace"
)

func TraceAnalyze(req tracecontract.TraceAnalyzeRequest) (tracecontract.TraceAnalyzeResult, error) {
	return (traceapp.Service{Effects: Source{ReadState: statestore.NewService().Read}}).Analyze(req)
}
