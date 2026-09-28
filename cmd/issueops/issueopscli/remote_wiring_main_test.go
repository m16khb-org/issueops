package issueopscli

import (
	"context"
	"os"
	"time"

	"issueops/cmd/issueops/issueopscli/remotecmd"
	issueopscore "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/provider"
	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	issuedomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// 프로덕션에서는 issueopsapp이 주입한다. 원격 CLI 계약 테스트는 실제 연산 경로를
// 검증하므로 같은 배선을 재현한다.
func wireRemoteForTests() {

	remotecmd.ConfigureRemote(remotecmd.RemoteDeps{
		CreateIssue: func(ctx context.Context, root string, cmd remoteapp.IssueCreateCommand, verify remoteapp.IssueLiveVerifier) (port.IssueProviderCreateIssueResult, error) {
			store := issueopscore.IssueCreateIntentStore{StateRoot: root}
			return remoteapp.NewIssueCreator(store, issueopscore.IssueCreationEnvironment{ResolveProvider: provider.Resolve}, remoteapp.NewTemplateBodyResolver(os.ReadFile), newIssueIntentsForTest(root), verify, time.Now).Create(ctx, cmd)
		},
		ResolveTemplateBody:  remoteapp.NewTemplateBodyResolver(os.ReadFile).Resolve,
		ReadScoreSummaryFile: remoteapp.NewTemplateBodyResolver(os.ReadFile).ScoreSummary,

		ReconcileIssueCreate: func(ctx context.Context, root, id string, confirm bool, verify remoteapp.IssueLiveVerifier) (issueopscontract.IssueOpsIssueCreateReconcileResult, error) {
			store := issueopscore.IssueCreateIntentStore{StateRoot: root}
			return remoteapp.NewIssueReconciler(store, issueopscore.IssueCreateCandidateSource{Resolve: provider.Resolve}, newIssueIntentsForTest(root), verify, time.Now).Reconcile(ctx, id, confirm)
		},

		CloseIssueOpsRemoteIssue: issueopscore.CloseIssueOpsRemoteIssue,
		CreateRemoteChild:        issueopscore.CreateRemoteChild,
		CreateRemotePullRequestWithHandler: func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error) {
			return issueopscore.CreateRemotePullRequestWithHandler(ctx, stateRoot, req, handler)
		},
		DecodeIssueOpsRemoteJudgeJSON:              issueopscore.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest:         issueopscore.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                          issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor:                 issueopscore.LinkIssueOpsChildWithActor,
		ObserveNativeProcessAncestry:               issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                               issueopscore.ReadIssueOps,
		ReflectDevilsAdvocateFindingsWithActor:     issueopscore.ReflectDevilsAdvocateFindingsWithActor,
		ReflectIssueCompletion:                     issueopscore.ReflectIssueCompletion,
		RenderIssueOpsRemoteJudgePrompt:            issueopscore.RenderIssueOpsRemoteJudgePrompt,
		ResolveRecordProvider:                      issuedomain.ResolveRecordProvider,
		ScoreIssueOpsRemoteCandidates:              issueopscore.ScoreIssueOpsRemoteCandidates,
		SyncRemoteIssueGraph:                       issueopscore.SyncRemoteIssueGraph,
		UmbrellaBranchGateReason:                   issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:              issueopscore.ValidateIssueOpsMutationActor,
		ValidateIssueOpsRemoteArtifactVerification: issueopscore.ValidateIssueOpsRemoteArtifactVerification,
		VerifyIssueOpsRemoteArtifactWithActor:      issueopscore.VerifyIssueOpsRemoteArtifactWithActor,
	})
}

func newIssueIntentsForTest(root string) *remoteapp.IssueCreateIntents {
	return remoteapp.NewIssueCreateIntents(issueopscore.IssueCreateIntentStore{StateRoot: root}, time.Now)
}
