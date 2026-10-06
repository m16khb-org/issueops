package issueopslease

import (
	"context"

	authorityapp "issueops/internal/application/authority"
	model "issueops/internal/contract/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
	authorityport "issueops/internal/port/authority"
	"issueops/internal/testsupport/authoritytest"
)

func leaseVerifier(inspect func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error)) authorityport.ActorVerifier {
	return authorityapp.New(nil, authoritytest.ProcessInspectorFunc(func(ctx context.Context, receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
		status, observed, err := inspect(ctx, leasedomain.ProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
		return status, model.NativeProcessReceipt{PID: observed.PID, StartedAt: observed.StartedAt, Executable: observed.Executable}, err
	}), nil, nil, nil, nil)
}
