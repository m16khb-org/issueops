package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"
	traceadapter "issueops/internal/adapter/trace"
	traceapp "issueops/internal/application/trace"
)

func newTraceService() traceapp.Service {
	state := newStateService(statestore.StateDir())
	return traceapp.Service{Effects: traceadapter.Source{ReadState: state.Read}}
}
