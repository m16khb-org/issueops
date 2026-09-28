package issueopsapp

import (
	"context"
	"os"

	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/provider"
	cycleapp "issueops/internal/application/issueopscycle"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/port"
)

func newPublicationCommand(root string, publish model.RemotePullRequestCreateHandler, observe application.AncestryObserver) *application.PublicationCommandService {
	var invoke application.PublicationInvoker
	if publish != nil {
		invoke = func(ctx context.Context, req model.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return publish(ctx, root, req)
		}
	}
	return application.NewPublicationCommandService(core.RemoteRecordStore{StateRoot: root}, application.NewTemplateBodyResolver(os.ReadFile), observe, func(actor model.NativeActor) (model.NativeActor, error) {
		return cycleapp.NormalizeNativeActor(actor, core.InspectNativeProcessReceipt)
	}, invoke)
}

func createPublication(ctx context.Context, root string, input application.PublicationInput, publish model.RemotePullRequestCreateHandler, observe application.AncestryObserver) (port.IssueProviderCreatePullRequestResult, error) {
	return newPublicationCommand(root, publish, observe).Create(ctx, input)
}

func newBodySyncCommand(root string, resolve func(string) (port.IssueProvider, error), observe application.AncestryObserver) *application.BodySyncCommandService {
	return application.NewBodySyncCommandService(core.RemoteRecordStore{StateRoot: root}, application.NewTemplateBodyResolver(os.ReadFile), func(name string) (application.BodySyncOperation, error) {
		prov, err := resolve(name)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, id string, cmd contract.Command, actor model.IssueOpsActor) (model.IssueOpsRecord, contract.Result, error) {
			return syncIssueOpsRemoteArtifactBody(ctx, root, id, cmd, prov, actor)
		}, nil
	}, observe)
}

func syncRemoteBody(ctx context.Context, root string, input application.BodySyncInput, observe application.AncestryObserver) (model.IssueOpsRecord, contract.Result, error) {
	return newBodySyncCommand(root, provider.Resolve, observe).Sync(ctx, input)
}
