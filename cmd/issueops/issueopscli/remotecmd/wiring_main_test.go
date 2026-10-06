package remotecmd

import (
	"context"
	executionissue "issueops/internal/contract/executionissue"
	"os"
	"time"

	issueopscore "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	"issueops/internal/adapter/provider"
	bodysyncapp "issueops/internal/application/issueopsbodysync"
	cycleapp "issueops/internal/application/issueopscycle"
	remoteapp "issueops/internal/application/issueopsremote"
	reportcontract "issueops/internal/contract/artifactreadability"
	issueopscontract "issueops/internal/contract/issueops"
	bodycontract "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/port"
)

// 프로덕션에서는 issueopsapp이 주입한다. 원격 CLI 테스트는 실제 연산 경로를
// 검증하므로 같은 배선을 재현한다.
func testRemoteCommand() Command {
	return Command{Operations: RemoteDeps{
		ReconcileChild: func(ctx context.Context, root string, cmd remoteapp.ChildReconcileCommand, observe remoteapp.AncestryObserver) (issueopscontract.ChildReconcileResult, error) {
			creatorIntents := &remoteapp.ChildCreateIntents{Store: issueopscore.RemoteRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueopscore.NativeActorVerifier()), Now: time.Now}
			service := remoteapp.ChildReconciler{Records: issueopscore.RemoteRecordStore{StateRoot: root}, Intents: creatorIntents, Resolve: func(name string) (port.IssueProviderChildCreateRecovery, error) {
				p, err := provider.Resolve(name)
				if err != nil {
					return nil, err
				}
				return p.(port.IssueProviderChildCreateRecovery), nil
			}}
			return service.Reconcile(ctx, cmd, observe)
		},
		CreateChild: func(ctx context.Context, root string, cmd remoteapp.ChildCreateCommand, observe remoteapp.AncestryObserver) (remoteapp.ChildCreateResult, error) {
			service := remoteapp.ChildCreator{Records: issueopscore.RemoteRecordStore{StateRoot: root}, Resolve: func(name string) (remoteapp.ChildProvider, error) { return provider.Resolve(name) }, Bodies: remoteapp.NewTemplateBodyResolver(os.ReadFile), Authorize: func(ctx context.Context, id string, actor issueopscontract.IssueOpsActor) error {
				return issueopscore.ValidateIssueOpsMutationActor(ctx, root, id, actor, issueopscore.NativeActorVerifier())
			}, Intents: &remoteapp.ChildCreateIntents{Store: issueopscore.RemoteRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueopscore.NativeActorVerifier()), Now: time.Now}, NewOperationID: issueopscore.RemotePublicationObserver{}.NewOperationID}

			return service.Create(ctx, cmd, observe)
		},
		VerifyRemoteArtifact: func(ctx context.Context, root, id string, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest, actor issueopscontract.IssueOpsActor, verify remoteapp.ArtifactLiveVerifier, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, error) {
			service := remoteapp.NewArtifactVerificationService(issueopscore.RemoteRecordStore{StateRoot: root}, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueopscore.NativeActorVerifier()), verify, observe, time.Now)
			return service.Verify(ctx, id, req, actor)
		},
		ReflectRemoteCompletion: func(ctx context.Context, root, id, providerOverride, resultBody string, confirm bool, verify remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, reportcontract.Report, error) {
			return newRemoteCompletionForTest(root, verify).Reflect(ctx, id, providerOverride, resultBody, confirm)
		},

		CloseRemoteIssue: func(ctx context.Context, root, id, providerOverride string, confirm bool, verify remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
			return newRemoteCompletionForTest(root, verify).Close(ctx, id, providerOverride, confirm)
		},

		CreateIssue: func(ctx context.Context, root string, cmd remoteapp.IssueCreateCommand, verify remoteapp.IssueLiveVerifier) (remoteapp.IssueCreateResult, error) {
			store := issueopscore.RemoteRecordStore{StateRoot: root}
			return remoteapp.NewIssueCreator(store, issueopscore.IssueCreationEnvironment{ResolveProvider: provider.Resolve}, remoteapp.NewTemplateBodyResolver(os.ReadFile), newIssueIntentsForTest(root), verify, time.Now).Create(ctx, cmd)
		},
		ReadScoreSummaryFile: remoteapp.NewTemplateBodyResolver(os.ReadFile).ScoreSummary,

		ReconcileIssueCreate: func(ctx context.Context, root, id string, confirm bool, verify remoteapp.IssueLiveVerifier) (issueopscontract.IssueOpsIssueCreateReconcileResult, error) {
			store := issueopscore.RemoteRecordStore{StateRoot: root}
			return remoteapp.NewIssueReconciler(store, issueopscore.IssueCreateCandidateSource{Resolve: provider.Resolve}, newIssueIntentsForTest(root), verify, time.Now).Reconcile(ctx, id, confirm)
		},

		CreatePublication: func(ctx context.Context, root string, input remoteapp.PublicationInput, handler issueopscontract.RemotePullRequestCreateHandler, observe remoteapp.AncestryObserver) (remoteapp.PublicationResult, error) {
			var invoke remoteapp.PublicationInvoker
			if handler != nil {
				invoke = func(ctx context.Context, req issueopscontract.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
					return handler(ctx, root, req)
				}
			}
			service := remoteapp.NewPublicationCommandService(issueopscore.RemoteRecordStore{StateRoot: root}, remoteapp.NewTemplateBodyResolver(os.ReadFile), observe, func(ctx context.Context, actor issueopscontract.NativeActor) (issueopscontract.NativeActor, error) {
				verified, err := issueopscore.NativeActorVerifier().Verify(ctx, actor)
				return verified.Identity, err
			}, invoke)
			return service.Create(ctx, input)
		},
		IssueOpsStateRoot:            issueOpsStateRootForTest,
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		ReflectReviewFindings: func(ctx context.Context, root, id, providerOverride string, confirm bool, actor issueopscontract.IssueOpsActor, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
			service := remoteapp.NewReviewReflectionService(issueopscore.RemoteRecordStore{StateRoot: root}, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueopscore.NativeActorVerifier()), func(name string) (remoteapp.ReviewReflectionProvider, error) { return provider.Resolve(name) }, observe, time.Now)
			return service.Reflect(ctx, id, providerOverride, confirm, actor)
		},
		SyncRemoteBody: func(ctx context.Context, root string, input remoteapp.BodySyncInput, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, bodycontract.Result, error) {
			service := remoteapp.NewBodySyncCommandService(issueopscore.RemoteRecordStore{StateRoot: root}, remoteapp.NewTemplateBodyResolver(os.ReadFile), func(name string) (remoteapp.BodySyncOperation, error) {
				prov, err := provider.Resolve(name)
				if err != nil {
					return nil, err
				}
				gateway, err := issueopscore.NewBodySyncProvider(prov)
				if err != nil {
					return nil, err
				}
				sync := bodysyncapp.NewService(issueopscore.BodySyncRepository{StateRoot: root}, gateway, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueopscore.NativeActorVerifier()), time.Now)
				return sync.Sync, nil
			}, observe)
			return service.Sync(ctx, input)
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
	return remoteapp.NewRemoteCompletionService(store, remoteapp.TrackedMaterials{Files: issueopscore.MaterialFiles{}}, remoteapp.NewCompletionReceipts(store, time.Now), func(name string) (remoteapp.CompletionProvider, error) { return provider.Resolve(name) }, verify)
}
