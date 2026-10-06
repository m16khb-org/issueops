package issueopsdecision

import (
	"context"

	authorityapp "issueops/internal/application/authority"
	model "issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
	"issueops/internal/testsupport/authoritytest"
)

func inspectorVerifier(inspect func(model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)) authorityport.ActorVerifier {
	return authorityapp.New(nil, authoritytest.ProcessInspectorFunc(func(_ context.Context, receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
		return inspect(receipt)
	}), nil, nil, nil, nil)
}

func liveVerifier() authorityport.ActorVerifier {
	return inspectorVerifier(func(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
		return "live", receipt, nil
	})
}
