package issueopsremote

import (
	"context"

	authorityapp "issueops/internal/application/authority"
	model "issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
)

func inspectorVerifier(inspect func(model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)) authorityport.ActorVerifier {
	return authorityapp.New(nil, authorityport.ProcessInspectorFunc(func(_ context.Context, receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
		return inspect(receipt)
	}), nil, nil, nil, nil)
}
