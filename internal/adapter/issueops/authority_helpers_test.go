package issueops

import (
	"context"
	"fmt"
	"sync"

	authorityapp "issueops/internal/application/authority"
	"issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
	"issueops/internal/testsupport/authoritytest"
)

func liveTestVerifier() authorityport.ActorVerifier {
	return authorityapp.New(nil, authoritytest.ProcessInspectorFunc(func(_ context.Context, receipt issueops.NativeProcessReceipt) (string, issueops.NativeProcessReceipt, error) {
		return NativeProcessStatusLive, receipt, nil
	}), nil, nil, nil, nil)
}

// liveFixtureReceipt is the real receipt of PID 1: live, and identical in every
// helper process, so native verification of fixture holders observes a live session.
var liveFixtureReceipt = sync.OnceValue(func() issueops.NativeProcessReceipt {
	receipt, err := ObserveNativeProcessReceipt(1)
	if err != nil {
		panic(fmt.Sprintf("observe live fixture receipt: %v", err))
	}
	return receipt
})
