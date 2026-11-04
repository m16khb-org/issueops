package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"
	traceadapter "issueops/internal/adapter/trace"
	traceapp "issueops/internal/application/trace"
)

func newTraceService() traceapp.Service {
	state := newStateService(statestore.Dir())
	return traceapp.Service{Effects: traceadapter.Source{ReadState: state.Read}}
}
