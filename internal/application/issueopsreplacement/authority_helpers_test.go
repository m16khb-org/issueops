package issueopsreplacement

import (
	"context"

	authorityapp "issueops/internal/application/authority"
	model "issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
)

func liveVerifier() authorityport.ActorVerifier {
	return authorityapp.New(nil, authorityport.ProcessInspectorFunc(func(_ context.Context, receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
		return "live", receipt, nil
	}), nil, nil, nil, nil)
}
