package basiccli

import (
	guardadapter "issueops/internal/adapter/guard"
	statestore "issueops/internal/adapter/outbound/state"
	traceadapter "issueops/internal/adapter/trace"
	traceapp "issueops/internal/application/trace"
	guardcontract "issueops/internal/contract/guard"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	GuardCheck = guardadapter.GuardCheck
	TraceAnalyze = (traceapp.Service{Effects: traceadapter.Source{ReadState: statestore.StateRead}}).Analyze
}

func init() {
	NewGuardBlockedError = func(findings []guardcontract.GuardFinding) error {
		return guardadapter.GuardBlockedError{Findings: findings}
	}
}
