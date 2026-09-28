package issueopsapp

import (
	"context"
	"os"

	"issueops/cmd/issueops/issueopscli/remotecmd"
	issueopscore "issueops/internal/adapter/issueops"
	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	issuedomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// remote CLI는 원격 연산 구현을 알지 않는다. 어댑터를 아는 곳은 composition
// root 하나뿐이다.
func configureIssueOpsRemote() {
	remotecmd.ConfigureRemote(remotecmd.RemoteDeps{
		ReflectRemoteCompletion: reflectRemoteCompletion,
		CloseRemoteIssue:        closeRemoteIssue,

		CreateIssue:          createIssue,
		ResolveTemplateBody:  remoteapp.NewTemplateBodyResolver(os.ReadFile).Resolve,
		ReadScoreSummaryFile: remoteapp.NewTemplateBodyResolver(os.ReadFile).ScoreSummary,
		ReconcileIssueCreate: reconcileIssueCreate,
		CreateRemoteChild:    issueopscore.CreateRemoteChild,
		CreateRemotePullRequestWithHandler: func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error) {
			return issueopscore.CreateRemotePullRequestWithHandler(ctx, stateRoot, req, handler)
		},
		DecodeIssueOpsRemoteJudgeJSON:              issueopscore.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest:         issueopscore.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                          issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor:                 issueopscore.LinkIssueOpsChildWithActor,
		ObserveNativeProcessAncestry:               issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                               issueopscore.ReadIssueOps,
		ReflectReviewFindings:                      reflectReviewFindings,
		RenderIssueOpsRemoteJudgePrompt:            issueopscore.RenderIssueOpsRemoteJudgePrompt,
		ResolveRecordProvider:                      issuedomain.ResolveRecordProvider,
		ScoreIssueOpsRemoteCandidates:              issueopscore.ScoreIssueOpsRemoteCandidates,
		SyncRemoteArtifactBody:                     syncIssueOpsRemoteArtifactBody,
		SyncRemoteIssueGraph:                       issueopscore.SyncRemoteIssueGraph,
		UmbrellaBranchGateReason:                   issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:              issueopscore.ValidateIssueOpsMutationActor,
		ValidateIssueOpsRemoteArtifactVerification: issueopscore.ValidateIssueOpsRemoteArtifactVerification,
		VerifyIssueOpsRemoteArtifactWithActor:      issueopscore.VerifyIssueOpsRemoteArtifactWithActor,
	})
}
