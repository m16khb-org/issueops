package issueopsapp

import (
	"context"
	"os"

	"issueops/cmd/issueops/issueopscli/remotecmd"
	issueopscore "issueops/internal/adapter/issueops"
	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	issuedomain "issueops/internal/domain/issueops"
	remotedomain "issueops/internal/domain/issueopsremote"
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
		DecodeIssueOpsRemoteJudgeJSON:      remotedomain.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest: remotedomain.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                  issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor: func(root, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return newIssueLinker(root).Child(context.Background(), id, childURL, title, &actor)
		},
		ObserveNativeProcessAncestry:    issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                    issueopscore.ReadIssueOps,
		ReflectReviewFindings:           reflectReviewFindings,
		RenderIssueOpsRemoteJudgePrompt: remotedomain.RenderIssueOpsRemoteJudgePrompt,
		ResolveRecordProvider:           issuedomain.ResolveRecordProvider,
		ScoreIssueOpsRemoteCandidates:   remotedomain.ScoreIssueOpsRemoteCandidates,
		SyncRemoteBody:                  syncRemoteBody,
		SyncIssueGraph:                  syncIssueGraph,
		UmbrellaBranchGateReason:        issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:   issueopscore.ValidateIssueOpsMutationActor,
	})
}
