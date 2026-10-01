package remotecmd

import (
	"context"

	remoteapp "issueops/internal/application/issueopsremote"
	reportcontract "issueops/internal/contract/artifactreadability"
	issueopscontract "issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/port"
)

type Command struct{ Operations RemoteDeps }
type RemoteDeps struct {
	ReconcileChild               func(context.Context, string, remoteapp.ChildReconcileCommand, remoteapp.AncestryObserver) (issueopscontract.ChildReconcileResult, error)
	CreateChild                  func(context.Context, string, remoteapp.ChildCreateCommand, remoteapp.AncestryObserver) (remoteapp.ChildCreateResult, error)
	VerifyRemoteArtifact         func(context.Context, string, string, issueopscontract.IssueOpsRemoteArtifactVerificationRequest, issueopscontract.IssueOpsActor, remoteapp.ArtifactLiveVerifier, remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, error)
	CloseRemoteIssue             func(context.Context, string, string, string, bool, remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error)
	ReflectRemoteCompletion      func(context.Context, string, string, string, string, bool, remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, reportcontract.Report, error)
	CreateIssue                  func(context.Context, string, remoteapp.IssueCreateCommand, remoteapp.IssueLiveVerifier) (remoteapp.IssueCreateResult, error)
	ReadScoreSummaryFile         func(string) (string, error)
	ReconcileIssueCreate         func(context.Context, string, string, bool, remoteapp.IssueLiveVerifier) (issueopscontract.IssueOpsIssueCreateReconcileResult, error)
	CreatePublication            func(context.Context, string, remoteapp.PublicationInput, issueopscontract.RemotePullRequestCreateHandler, remoteapp.AncestryObserver) (remoteapp.PublicationResult, error)
	IssueOpsStateRoot            func() string
	ObserveNativeProcessAncestry func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	ReflectReviewFindings        func(ctx context.Context, stateRoot, id, providerOverride string, confirm bool, actor issueopscontract.IssueOpsActor, observe remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error)
	SyncRemoteBody               func(context.Context, string, remoteapp.BodySyncInput, remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, bodysynccontract.Result, error)
	SyncIssueGraph               func(context.Context, string, string, bool) (map[string]any, error)
}
