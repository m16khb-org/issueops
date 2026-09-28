package issueopsapp

import (
	"os"

	"issueops/cmd/issueops/issueopscli/remotecmd"
	issueopscore "issueops/internal/adapter/issueops"
	remoteapp "issueops/internal/application/issueopsremote"
	issuedomain "issueops/internal/domain/issueops"
)

// remote CLI는 원격 연산 구현을 알지 않는다. 어댑터를 아는 곳은 composition
// root 하나뿐이다.
func configureIssueOpsRemote() {
	remotecmd.ConfigureRemote(remotecmd.RemoteDeps{
		VerifyRemoteArtifact:    verifyRemoteArtifact,
		ReflectRemoteCompletion: reflectRemoteCompletion,
		CloseRemoteIssue:        closeRemoteIssue,

		CreateIssue:                        createIssue,
		ResolveTemplateBody:                remoteapp.NewTemplateBodyResolver(os.ReadFile).Resolve,
		ReadScoreSummaryFile:               remoteapp.NewTemplateBodyResolver(os.ReadFile).ScoreSummary,
		ReconcileIssueCreate:               reconcileIssueCreate,
		CreateRemoteChild:                  issueopscore.CreateRemoteChild,
		CreatePublication:                  createPublication,
		DecodeIssueOpsRemoteJudgeJSON:      issueopscore.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest: issueopscore.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                  issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor:         issueopscore.LinkIssueOpsChildWithActor,
		ObserveNativeProcessAncestry:       issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                       issueopscore.ReadIssueOps,
		ReflectReviewFindings:              reflectReviewFindings,
		RenderIssueOpsRemoteJudgePrompt:    issueopscore.RenderIssueOpsRemoteJudgePrompt,
		ResolveRecordProvider:              issuedomain.ResolveRecordProvider,
		ScoreIssueOpsRemoteCandidates:      issueopscore.ScoreIssueOpsRemoteCandidates,
		SyncRemoteBody:                     syncRemoteBody,
		SyncRemoteIssueGraph:               issueopscore.SyncRemoteIssueGraph,
		UmbrellaBranchGateReason:           issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:      issueopscore.ValidateIssueOpsMutationActor,
	})
}
