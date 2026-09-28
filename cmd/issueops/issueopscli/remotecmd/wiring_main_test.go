package remotecmd

import (
	"context"
	"os"
	"testing"
	"time"

	issueopscore "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	"issueops/internal/adapter/provider"
	cycleapp "issueops/internal/application/issueopscycle"
	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	issuedomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// 프로덕션에서는 issueopsapp이 주입한다. 원격 CLI 테스트는 실제 연산 경로를
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	ConfigureRemote(RemoteDeps{
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
		ResolveTemplateBody:  remoteapp.NewTemplateBodyResolver(os.ReadFile).Resolve,
		ReadScoreSummaryFile: remoteapp.NewTemplateBodyResolver(os.ReadFile).ScoreSummary,

		ReconcileIssueCreate: func(ctx context.Context, root, id string, confirm bool, verify remoteapp.IssueLiveVerifier) (issueopscontract.IssueOpsIssueCreateReconcileResult, error) {
			store := issueopscore.RemoteRecordStore{StateRoot: root}
			return remoteapp.NewIssueReconciler(store, issueopscore.IssueCreateCandidateSource{Resolve: provider.Resolve}, newIssueIntentsForTest(root), verify, time.Now).Reconcile(ctx, id, confirm)
		},

		CreateRemoteChild: issueopscore.CreateRemoteChild,
		CreatePublication: func(ctx context.Context, root string, input remoteapp.PublicationInput, handler issueopscontract.RemotePullRequestCreateHandler, observe remoteapp.AncestryObserver) (port.IssueProviderCreatePullRequestResult, error) {
			service := remoteapp.NewPublicationCommandService(issueopscore.RemoteRecordStore{StateRoot: root}, remoteapp.NewTemplateBodyResolver(os.ReadFile), observe, func(ctx context.Context, req issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
				return issueopscore.CreateRemotePullRequestWithHandler(ctx, root, req, handler)
			})
			return service.Create(ctx, input)
		},
		DecodeIssueOpsRemoteJudgeJSON:      issueopscore.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest: issueopscore.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                  issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor:         issueopscore.LinkIssueOpsChildWithActor,
		ObserveNativeProcessAncestry:       issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                       issueopscore.ReadIssueOps,
		ReflectReviewFindings: func(ctx context.Context, root, id, providerOverride string, confirm bool, actor issueopscontract.IssueOpsActor, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
			service := remoteapp.NewReviewReflectionService(issueopscore.RemoteRecordStore{StateRoot: root}, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), func(name string) (remoteapp.ReviewReflectionProvider, error) { return provider.Resolve(name) }, observe, time.Now)
			return service.Reflect(ctx, id, providerOverride, confirm, actor)
		},
		RenderIssueOpsRemoteJudgePrompt: issueopscore.RenderIssueOpsRemoteJudgePrompt,
		ResolveRecordProvider:           issuedomain.ResolveRecordProvider,
		ScoreIssueOpsRemoteCandidates:   issueopscore.ScoreIssueOpsRemoteCandidates,
		SyncRemoteIssueGraph:            issueopscore.SyncRemoteIssueGraph,
		UmbrellaBranchGateReason:        issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:   issueopscore.ValidateIssueOpsMutationActor,
	})
	os.Exit(m.Run())
}

func newIssueIntentsForTest(root string) *remoteapp.IssueCreateIntents {
	return remoteapp.NewIssueCreateIntents(issueopscore.RemoteRecordStore{StateRoot: root}, time.Now)
}

func newRemoteCompletionForTest(root string, verify remoteapp.MergeVerifier) *remoteapp.RemoteCompletionService {
	store := issueopscore.RemoteRecordStore{StateRoot: root}
	return remoteapp.NewRemoteCompletionService(store, remoteapp.NewCompletionCollector(issueopscore.CompletionArtifacts{}), remoteapp.NewCompletionReceipts(store, time.Now), func(name string) (remoteapp.CompletionProvider, error) { return provider.Resolve(name) }, verify)
}
