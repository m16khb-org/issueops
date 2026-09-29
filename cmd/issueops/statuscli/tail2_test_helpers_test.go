package statuscli

import (
	basicclit2d "issueops/cmd/issueops/basiccli"
	guardadapter "issueops/internal/adapter/guard"
	statestore "issueops/internal/adapter/outbound/state"
	traceadapter "issueops/internal/adapter/trace"
	traceapp "issueops/internal/application/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	basicclit2d.GuardCheck = guardadapter.GuardCheck
	basicclit2d.TraceAnalyze = (traceapp.Service{Effects: traceadapter.Source{ReadState: statestore.StateRead}}).Analyze
}
