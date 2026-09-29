package mcpcli

import (
	basicclit2d "issueops/cmd/issueops/basiccli"
	commitsuggestadapter "issueops/internal/adapter/commitsuggest"
	guardadapter "issueops/internal/adapter/guard"
	lintdiagnoseadapter "issueops/internal/adapter/lintdiagnose"
	statestore "issueops/internal/adapter/outbound/state"
	traceadapter "issueops/internal/adapter/trace"
	traceapp "issueops/internal/application/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	SuggestCommit = commitsuggestadapter.SuggestCommit
	basicclit2d.GuardCheck = guardadapter.GuardCheck
	basicclit2d.TraceAnalyze = (traceapp.Service{Effects: traceadapter.Source{ReadState: statestore.StateRead}}).Analyze
}
