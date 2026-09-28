package issueopsapp

import (
	"context"
	"issueops/cmd/issueops/issueopscli/remotecmd"

	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// remote CLI는 원격 연산 구현을 알지 않는다. 어댑터를 아는 곳은 composition
// root 하나뿐이다.
func configureIssueOpsRemote() {
	remotecmd.ConfigureRemote(remotecmd.RemoteDeps{
		BeginIssueCreateIntent:    beginIssueCreateIntent,
		CloseIssueOpsRemoteIssue:  issueopscore.CloseIssueOpsRemoteIssue,
		CompleteIssueCreateIntent: completeIssueCreateIntent,
		CreateRemoteChild:         issueopscore.CreateRemoteChild,
		CreateRemoteIssue:         issueopscore.CreateRemoteIssue,
		CreateRemoteIssueContext:  issueopscore.CreateRemoteIssueContext,
		CreateRemotePullRequestWithHandler: func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error) {
			return issueopscore.CreateRemotePullRequestWithHandler(ctx, stateRoot, req, handler)
		},
		DecodeIssueOpsRemoteJudgeJSON:              issueopscore.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest:         issueopscore.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                          issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor:                 issueopscore.LinkIssueOpsChildWithActor,
		ObserveNativeProcessAncestry:               issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                               issueopscore.ReadIssueOps,
		RecordIssueCreateOutcome:                   recordIssueCreateOutcome,
		ReflectDevilsAdvocateFindingsWithActor:     issueopscore.ReflectDevilsAdvocateFindingsWithActor,
		ReflectIssueCompletion:                     issueopscore.ReflectIssueCompletion,
		RenderIssueOpsRemoteJudgePrompt:            issueopscore.RenderIssueOpsRemoteJudgePrompt,
		InferProviderFromRepoRemotes:               issueopscore.InferProviderFromRepoRemotes,
		ResolveRecordProvider:                      issueopscore.ResolveRecordProvider,
		ResolveProviderProjectAuthority:            issueopscore.ResolveProviderProjectAuthority,
		ScoreIssueOpsRemoteCandidates:              issueopscore.ScoreIssueOpsRemoteCandidates,
		SyncRemoteArtifactBody:                     syncIssueOpsRemoteArtifactBody,
		SyncRemoteIssueGraph:                       issueopscore.SyncRemoteIssueGraph,
		UmbrellaBranchGateReason:                   issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:              issueopscore.ValidateIssueOpsMutationActor,
		ValidateIssueOpsRemoteArtifactVerification: issueopscore.ValidateIssueOpsRemoteArtifactVerification,
		VerifyIssueOpsRemoteArtifactWithActor:      issueopscore.VerifyIssueOpsRemoteArtifactWithActor,
	})
}
