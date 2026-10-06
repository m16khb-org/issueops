package issueopsdecision

import (
	"context"

	authorityapp "issueops/internal/application/authority"
	model "issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
	"issueops/internal/testsupport/authoritytest"
)

func liveVerifier() authorityport.ActorVerifier {
	return authorityapp.New(nil, authoritytest.ProcessInspectorFunc(func(_ context.Context, receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
		return "live", receipt, nil
	}), nil, nil, nil, nil)
}
