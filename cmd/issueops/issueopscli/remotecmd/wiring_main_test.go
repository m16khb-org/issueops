package remotecmd

import (
	"context"
	"os"
	"time"

	issueopscore "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	"issueops/internal/adapter/provider"
	cycleapp "issueops/internal/application/issueopscycle"
	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// 프로덕션에서는 issueopsapp이 주입한다. 원격 CLI 테스트는 실제 연산 경로를
// 검증하므로 같은 배선을 재현한다.
func testRemoteCommand() Command {
	return Command{Operations: RemoteDeps{
		CreateChild: func(ctx context.Context, root string, cmd remoteapp.ChildCreateCommand, observe remoteapp.AncestryObserver) (port.IssueProviderCreateChildResult, error) {
			service := remoteapp.ChildCreator{Records: issueopscore.RemoteRecordStore{StateRoot: root}, Resolve: func(name string) (remoteapp.ChildProvider, error) { return provider.Resolve(name) }, Bodies: remoteapp.NewTemplateBodyResolver(os.ReadFile), Authorize: func(_ context.Context, id string, actor issueopscontract.IssueOpsActor) error {
				return issueopscore.ValidateIssueOpsMutationActor(root, id, actor)
			}, Link: func(ctx context.Context, id, url, title string, actor issueopscontract.IssueOpsActor) error {
				_, err := issueLinkerForTest(root).Child(ctx, id, url, title, &actor)
				return err
			}}
			return service.Create(ctx, cmd, observe)
		},
		VerifyRemoteArtifact: func(ctx context.Context, root, id string, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest, actor issueopscontract.IssueOpsActor, verify remoteapp.ArtifactLiveVerifier, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, error) {
			service := remoteapp.NewArtifactVerificationService(issueopscore.RemoteRecordStore{StateRoot: root}, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), verify, observe, time.Now)
			return service.Verify(ctx, id, req, actor)
		},
		ReflectRemoteCompletion: func(ctx context.Context, root, id, providerOverride string, confirm bool, verify remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
			return newRemoteCompletionForTest(root, verify).Reflect(ctx, id, providerOverride, confirm)
		},

		CloseRemoteIssue: func(ctx context.Context, root, id, providerOverride string, confirm bool, verify remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
			return newRemoteCompletionForTest(root, verify).Close(ctx, id, providerOverride, confirm)
		},

		CreateIssue: func(ctx context.Context, root string, cmd remoteapp.IssueCreateCommand, verify remoteapp.IssueLiveVerifier) (port.IssueProviderCreateIssueResult, error) {
			store := issueopscore.RemoteRecordStore{StateRoot: root}
			return remoteapp.NewIssueCreator(store, issueopscore.IssueCreationEnvironment{ResolveProvider: provider.Resolve}, remoteapp.NewTemplateBodyResolver(os.ReadFile), newIssueIntentsForTest(root), verify, time.Now).Create(ctx, cmd)
		},
		ReadScoreSummaryFile: remoteapp.NewTemplateBodyResolver(os.ReadFile).ScoreSummary,

		ReconcileIssueCreate: func(ctx context.Context, root, id string, confirm bool, verify remoteapp.IssueLiveVerifier) (issueopscontract.IssueOpsIssueCreateReconcileResult, error) {
			store := issueopscore.RemoteRecordStore{StateRoot: root}
			return remoteapp.NewIssueReconciler(store, issueopscore.IssueCreateCandidateSource{Resolve: provider.Resolve}, newIssueIntentsForTest(root), verify, time.Now).Reconcile(ctx, id, confirm)
		},

		CreatePublication: func(ctx context.Context, root string, input remoteapp.PublicationInput, handler issueopscontract.RemotePullRequestCreateHandler, observe remoteapp.AncestryObserver) (port.IssueProviderCreatePullRequestResult, error) {
			var invoke remoteapp.PublicationInvoker
			if handler != nil {
				invoke = func(ctx context.Context, req issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
					return handler(ctx, root, req)
				}
			}
			service := remoteapp.NewPublicationCommandService(issueopscore.RemoteRecordStore{StateRoot: root}, remoteapp.NewTemplateBodyResolver(os.ReadFile), observe, func(actor issueopscontract.NativeActor) (issueopscontract.NativeActor, error) {
				return cycleapp.NormalizeNativeActor(actor, issueopscore.InspectNativeProcessReceipt)
			}, invoke)
			return service.Create(ctx, input)
		},
		IssueOpsStateRoot:            issueopscore.IssueOpsStateRoot,
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		ReflectReviewFindings: func(ctx context.Context, root, id, providerOverride string, confirm bool, actor issueopscontract.IssueOpsActor, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
			service := remoteapp.NewReviewReflectionService(issueopscore.RemoteRecordStore{StateRoot: root}, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), func(name string) (remoteapp.ReviewReflectionProvider, error) { return provider.Resolve(name) }, observe, time.Now)
			return service.Reflect(ctx, id, providerOverride, confirm, actor)
		},
		SyncIssueGraph: func(ctx context.Context, root, id string, confirm bool) (map[string]any, error) {
			return remoteapp.NewIssueGraphSyncService(issueopscore.RemoteRecordStore{StateRoot: root}, issueopscore.IssueGraphPoster{}).Sync(ctx, id, confirm)
		},
	}}
}

func newIssueIntentsForTest(root string) *remoteapp.IssueCreateIntents {
	return remoteapp.NewIssueCreateIntents(issueopscore.RemoteRecordStore{StateRoot: root}, time.Now)
}

func newRemoteCompletionForTest(root string, verify remoteapp.MergeVerifier) *remoteapp.RemoteCompletionService {
	store := issueopscore.RemoteRecordStore{StateRoot: root}
	return remoteapp.NewRemoteCompletionService(store, remoteapp.NewCompletionCollector(issueopscore.CompletionArtifacts{}), remoteapp.NewCompletionReceipts(store, time.Now), func(name string) (remoteapp.CompletionProvider, error) { return provider.Resolve(name) }, verify)
}
