package remotecmd

import (
	"context"
	"errors"

	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
	issueopsremote "issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
)

var errRemoteNotConfigured = errors.New("issueops remote is not configured")

// 원격 이슈·PR 연산은 외부 제공자와 상태 저장소를 다루는 I/O다. CLI는 그 구현을
// 모르고 composition root가 주입한 함수만 호출한다.
var remoteDeps = neutralRemoteDeps()

// RemoteDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type RemoteDeps struct {
	VerifyRemoteArtifact               func(context.Context, string, string, issueopscontract.IssueOpsRemoteArtifactVerificationRequest, issueopscontract.IssueOpsActor, remoteapp.ArtifactLiveVerifier, remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, error)
	CloseRemoteIssue                   func(context.Context, string, string, string, bool, remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error)
	ReflectRemoteCompletion            func(context.Context, string, string, string, bool, remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error)
	CreateIssue                        func(context.Context, string, remoteapp.IssueCreateCommand, remoteapp.IssueLiveVerifier) (port.IssueProviderCreateIssueResult, error)
	ResolveTemplateBody                func(remoteapp.TemplateBodyRequest) (string, error)
	ReadScoreSummaryFile               func(string) (string, error)
	ReconcileIssueCreate               func(context.Context, string, string, bool, remoteapp.IssueLiveVerifier) (issueopscontract.IssueOpsIssueCreateReconcileResult, error)
	CreateRemoteChild                  func(req port.IssueProviderCreateChildRequest, prov port.IssueProvider) (port.IssueProviderCreateChildResult, error)
	CreatePublication                  func(context.Context, string, remoteapp.PublicationInput, issueopscontract.RemotePullRequestCreateHandler, remoteapp.AncestryObserver) (port.IssueProviderCreatePullRequestResult, error)
	DecodeIssueOpsRemoteJudgeJSON      func(out []byte) (issueopsremote.IssueOpsRemoteScoringResult, error)
	DecodeIssueOpsRemoteScoringRequest func(data []byte) (issueopsremote.IssueOpsRemoteScoringRequest, error)
	IssueOpsStateRoot                  func() string
	LinkIssueOpsChildWithActor         func(stateRoot, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ObserveNativeProcessAncestry       func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	ReadIssueOps                       func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	ReflectReviewFindings              func(ctx context.Context, stateRoot, id, providerOverride string, confirm bool, actor issueopscontract.IssueOpsActor, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error)
	RenderIssueOpsRemoteJudgePrompt    func(req issueopsremote.IssueOpsRemoteLLMJudgeRequest) (issueopsremote.IssueOpsRemoteJudgePromptResult, error)
	ResolveRecordProvider              func(record issueopscontract.IssueOpsRecord) string
	ScoreIssueOpsRemoteCandidates      func(req issueopsremote.IssueOpsRemoteScoringRequest) (issueopsremote.IssueOpsRemoteScoringResult, error)
	SyncRemoteBody                     func(context.Context, string, remoteapp.BodySyncInput, remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, bodysynccontract.Result, error)
	SyncRemoteIssueGraph               func(record issueopscontract.IssueOpsRecord) (map[string]any, error)
	UmbrellaBranchGateReason           func(record issueopscontract.IssueOpsRecord) string
	ValidateIssueOpsMutationActor      func(stateRoot, id string, actor issueopscontract.IssueOpsActor) error
}

// ConfigureRemote는 composition root가 실제 구현을 꽂는 진입점이다.
func ConfigureRemote(deps RemoteDeps) {
	if deps.CloseRemoteIssue != nil {
		remoteDeps.CloseRemoteIssue = deps.CloseRemoteIssue
	}
	if deps.ReflectRemoteCompletion != nil {
		remoteDeps.ReflectRemoteCompletion = deps.ReflectRemoteCompletion
	}
	if deps.CreateIssue != nil {
		remoteDeps.CreateIssue = deps.CreateIssue
	}
	if deps.ResolveTemplateBody != nil {
		remoteDeps.ResolveTemplateBody = deps.ResolveTemplateBody
	}
	if deps.ReadScoreSummaryFile != nil {
		remoteDeps.ReadScoreSummaryFile = deps.ReadScoreSummaryFile
	}
	if deps.ReconcileIssueCreate != nil {
		remoteDeps.ReconcileIssueCreate = deps.ReconcileIssueCreate
	}
	if deps.CreateRemoteChild != nil {
		remoteDeps.CreateRemoteChild = deps.CreateRemoteChild
	}
	if deps.CreatePublication != nil {
		remoteDeps.CreatePublication = deps.CreatePublication
	}
	if deps.DecodeIssueOpsRemoteJudgeJSON != nil {
		remoteDeps.DecodeIssueOpsRemoteJudgeJSON = deps.DecodeIssueOpsRemoteJudgeJSON
	}
	if deps.DecodeIssueOpsRemoteScoringRequest != nil {
		remoteDeps.DecodeIssueOpsRemoteScoringRequest = deps.DecodeIssueOpsRemoteScoringRequest
	}
	if deps.IssueOpsStateRoot != nil {
		remoteDeps.IssueOpsStateRoot = deps.IssueOpsStateRoot
	}
	if deps.LinkIssueOpsChildWithActor != nil {
		remoteDeps.LinkIssueOpsChildWithActor = deps.LinkIssueOpsChildWithActor
	}
	if deps.ObserveNativeProcessAncestry != nil {
		remoteDeps.ObserveNativeProcessAncestry = deps.ObserveNativeProcessAncestry
	}
	if deps.ReadIssueOps != nil {
		remoteDeps.ReadIssueOps = deps.ReadIssueOps
	}
	if deps.ReflectReviewFindings != nil {
		remoteDeps.ReflectReviewFindings = deps.ReflectReviewFindings
	}
	if deps.RenderIssueOpsRemoteJudgePrompt != nil {
		remoteDeps.RenderIssueOpsRemoteJudgePrompt = deps.RenderIssueOpsRemoteJudgePrompt
	}
	if deps.ResolveRecordProvider != nil {
		remoteDeps.ResolveRecordProvider = deps.ResolveRecordProvider
	}
	if deps.ScoreIssueOpsRemoteCandidates != nil {
		remoteDeps.ScoreIssueOpsRemoteCandidates = deps.ScoreIssueOpsRemoteCandidates
	}
	if deps.SyncRemoteBody != nil {
		remoteDeps.SyncRemoteBody = deps.SyncRemoteBody
	}
	if deps.SyncRemoteIssueGraph != nil {
		remoteDeps.SyncRemoteIssueGraph = deps.SyncRemoteIssueGraph
	}
	if deps.UmbrellaBranchGateReason != nil {
		remoteDeps.UmbrellaBranchGateReason = deps.UmbrellaBranchGateReason
	}
	if deps.ValidateIssueOpsMutationActor != nil {
		remoteDeps.ValidateIssueOpsMutationActor = deps.ValidateIssueOpsMutationActor
	}
	if deps.VerifyRemoteArtifact != nil {
		remoteDeps.VerifyRemoteArtifact = deps.VerifyRemoteArtifact
	}
}

// 배선 누락이 패닉이 아니라 명시적 오류로 드러나도록 중립 기본값을 둔다.
func neutralRemoteDeps() RemoteDeps {
	return RemoteDeps{
		CloseRemoteIssue: func(context.Context, string, string, string, bool, remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
			return issueopscontract.IssueOpsRecord{}, port.IssueProviderCloseIssueResult{}, errRemoteNotConfigured
		},
		ReflectRemoteCompletion: func(context.Context, string, string, string, bool, remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
			return issueopscontract.IssueOpsRecord{}, port.IssueProviderUpdateIssueBodySectionResult{}, errRemoteNotConfigured
		},
		CreateIssue: func(context.Context, string, remoteapp.IssueCreateCommand, remoteapp.IssueLiveVerifier) (port.IssueProviderCreateIssueResult, error) {
			return port.IssueProviderCreateIssueResult{}, errRemoteNotConfigured
		},
		ResolveTemplateBody:  func(remoteapp.TemplateBodyRequest) (string, error) { return "", errRemoteNotConfigured },
		ReadScoreSummaryFile: func(string) (string, error) { return "", errRemoteNotConfigured },
		ReconcileIssueCreate: func(context.Context, string, string, bool, remoteapp.IssueLiveVerifier) (issueopscontract.IssueOpsIssueCreateReconcileResult, error) {
			return issueopscontract.IssueOpsIssueCreateReconcileResult{}, errRemoteNotConfigured
		},
		CreateRemoteChild: func(req port.IssueProviderCreateChildRequest, prov port.IssueProvider) (port.IssueProviderCreateChildResult, error) {
			return port.IssueProviderCreateChildResult{}, errRemoteNotConfigured
		},
		CreatePublication: func(context.Context, string, remoteapp.PublicationInput, issueopscontract.RemotePullRequestCreateHandler, remoteapp.AncestryObserver) (port.IssueProviderCreatePullRequestResult, error) {
			return port.IssueProviderCreatePullRequestResult{}, errRemoteNotConfigured
		},
		DecodeIssueOpsRemoteJudgeJSON: func(out []byte) (issueopsremote.IssueOpsRemoteScoringResult, error) {
			return issueopsremote.IssueOpsRemoteScoringResult{}, errRemoteNotConfigured
		},
		DecodeIssueOpsRemoteScoringRequest: func(data []byte) (issueopsremote.IssueOpsRemoteScoringRequest, error) {
			return issueopsremote.IssueOpsRemoteScoringRequest{}, errRemoteNotConfigured
		},
		IssueOpsStateRoot: func() string { return "" },
		LinkIssueOpsChildWithActor: func(stateRoot, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
		ObserveNativeProcessAncestry: func(pid int) ([]issueopscontract.NativeProcessReceipt, error) { return nil, errRemoteNotConfigured },
		ReadIssueOps: func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
		ReflectReviewFindings: func(ctx context.Context, stateRoot, id, providerOverride string, confirm bool, actor issueopscontract.IssueOpsActor, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
			return issueopscontract.IssueOpsRecord{}, port.IssueProviderUpdateIssueBodySectionResult{}, errRemoteNotConfigured
		},
		RenderIssueOpsRemoteJudgePrompt: func(req issueopsremote.IssueOpsRemoteLLMJudgeRequest) (issueopsremote.IssueOpsRemoteJudgePromptResult, error) {
			return issueopsremote.IssueOpsRemoteJudgePromptResult{}, errRemoteNotConfigured
		},
		ResolveRecordProvider: func(record issueopscontract.IssueOpsRecord) string { return "" },
		ScoreIssueOpsRemoteCandidates: func(req issueopsremote.IssueOpsRemoteScoringRequest) (issueopsremote.IssueOpsRemoteScoringResult, error) {
			return issueopsremote.IssueOpsRemoteScoringResult{}, errRemoteNotConfigured
		},
		SyncRemoteBody: func(context.Context, string, remoteapp.BodySyncInput, remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, bodysynccontract.Result, error) {
			return issueopscontract.IssueOpsRecord{}, bodysynccontract.Result{}, errRemoteNotConfigured
		},
		SyncRemoteIssueGraph: func(record issueopscontract.IssueOpsRecord) (map[string]any, error) {
			return nil, errRemoteNotConfigured
		},
		UmbrellaBranchGateReason:      func(record issueopscontract.IssueOpsRecord) string { return "" },
		ValidateIssueOpsMutationActor: func(stateRoot, id string, actor issueopscontract.IssueOpsActor) error { return errRemoteNotConfigured },
		VerifyRemoteArtifact: func(context.Context, string, string, issueopscontract.IssueOpsRemoteArtifactVerificationRequest, issueopscontract.IssueOpsActor, remoteapp.ArtifactLiveVerifier, remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
	}
}
