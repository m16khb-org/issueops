package remotecmd

import (
	"context"
	"time"

	issueopscore "issueops/internal/adapter/issueops"
	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"os"
	"testing"
)

// 프로덕션에서는 issueopsapp이 주입한다. 원격 CLI 테스트는 실제 연산 경로를
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	ConfigureRemote(RemoteDeps{
		BeginIssueCreateIntent: func(root, id string, request issueopscontract.IssueOpsIssueCreateIntentRequest) (issueopscontract.IssueOpsRecord, error) {
			return newIssueIntentsForTest(root).Begin(context.Background(), id, request)
		},
		CloseIssueOpsRemoteIssue: issueopscore.CloseIssueOpsRemoteIssue,
		CompleteIssueCreateIntent: func(root, id, url, at string) (issueopscontract.IssueOpsRecord, error) {
			return newIssueIntentsForTest(root).Complete(context.Background(), id, url, at)
		},
		CreateRemoteChild:        issueopscore.CreateRemoteChild,
		CreateRemoteIssue:        issueopscore.CreateRemoteIssue,
		CreateRemoteIssueContext: issueopscore.CreateRemoteIssueContext,
		CreateRemotePullRequestWithHandler: func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error) {
			return issueopscore.CreateRemotePullRequestWithHandler(ctx, stateRoot, req, handler)
		},
		DecodeIssueOpsRemoteJudgeJSON:      issueopscore.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest: issueopscore.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                  issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor:         issueopscore.LinkIssueOpsChildWithActor,
		ObserveNativeProcessAncestry:       issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                       issueopscore.ReadIssueOps,
		RecordIssueCreateOutcome: func(root, id string, outcome issueopscontract.IssueOpsIssueCreateOutcome) (issueopscontract.IssueOpsRecord, error) {
			return newIssueIntentsForTest(root).Outcome(context.Background(), id, outcome)
		},
		ReflectDevilsAdvocateFindingsWithActor:     issueopscore.ReflectDevilsAdvocateFindingsWithActor,
		ReflectIssueCompletion:                     issueopscore.ReflectIssueCompletion,
		RenderIssueOpsRemoteJudgePrompt:            issueopscore.RenderIssueOpsRemoteJudgePrompt,
		ResolveRecordProvider:                      issueopscore.ResolveRecordProvider,
		ResolveProviderProjectAuthority:            issueopscore.ResolveProviderProjectAuthority,
		ScoreIssueOpsRemoteCandidates:              issueopscore.ScoreIssueOpsRemoteCandidates,
		SyncRemoteIssueGraph:                       issueopscore.SyncRemoteIssueGraph,
		UmbrellaBranchGateReason:                   issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:              issueopscore.ValidateIssueOpsMutationActor,
		ValidateIssueOpsRemoteArtifactVerification: issueopscore.ValidateIssueOpsRemoteArtifactVerification,
		VerifyIssueOpsRemoteArtifactWithActor:      issueopscore.VerifyIssueOpsRemoteArtifactWithActor,
	})
	os.Exit(m.Run())
}

func newIssueIntentsForTest(root string) *remoteapp.IssueCreateIntents {
	return remoteapp.NewIssueCreateIntents(issueopscore.IssueCreateIntentStore{StateRoot: root}, time.Now)
}
