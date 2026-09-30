package orphancleanup

import (
	"context"
	adapter "issueops/internal/adapter/issueops"
	healthadapter "issueops/internal/adapter/operationalhealth"
	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsorphancleanup"
	health "issueops/internal/contract/operationalhealth"
)

type Request = contract.Request
type Result = contract.Result
type ApplyRequest = contract.ApplyRequest

type Dependencies struct {
	Collect      func(context.Context, string) (health.Snapshot, error)
	VerifyMerged func(model.IssueOpsRemoteArtifactVerification) error
}

func cleaner(deps Dependencies) app.OrphanCleaner {
	environment := adapter.OrphanEnvironment{StateRoot: issueOpsStateRootForTest()}
	collector := healthadapter.Collector{Git: environment, IssueOps: healthadapter.IssueOpsReader{StateRoot: environment.StateRoot, ListIDs: adapter.ListIssueOpsIDs, ListLeaseHolders: adapter.ListLeaseHolderIndexes, Read: adapter.ReadIssueOpsExisting}, InspectNativeProcess: adapter.InspectNativeProcessReceipt}
	environment.LocalInventory = func(ctx context.Context, repo string) (health.Snapshot, error) {
		return collector.CollectLocal(ctx, repo), nil
	}
	return app.OrphanCleaner{Environment: environment, Collect: deps.Collect, VerifyMerged: func(ctx context.Context, artifact model.IssueOpsRemoteArtifactVerification) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return deps.VerifyMerged(artifact)
	}}
}
func Preview(ctx context.Context, request Request, deps Dependencies) (Result, error) {
	return cleaner(deps).Preview(ctx, request)
}
func Apply(ctx context.Context, request Request, apply ApplyRequest, deps Dependencies) (Result, error) {
	return cleaner(deps).Apply(ctx, request, apply)
}
