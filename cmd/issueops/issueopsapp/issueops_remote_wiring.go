package issueopsapp

import (
	"context"
	model "issueops/internal/contract/issueops"
	"os"

	"issueops/cmd/issueops/issueopscli/remotecmd"
	issueopscore "issueops/internal/adapter/issueops"
	remoteapp "issueops/internal/application/issueopsremote"
)

// remote CLI는 원격 연산 구현을 알지 않는다. 어댑터를 아는 곳은 composition
// root 하나뿐이다.
func newIssueOpsRemote(root string) remotecmd.Command {
	return remotecmd.Command{Operations: remotecmd.RemoteDeps{
		ReconcileChild: func(ctx context.Context, root string, cmd remoteapp.ChildReconcileCommand, observe remoteapp.AncestryObserver) (model.ChildReconcileResult, error) {
			return newChildReconciler(root).Reconcile(ctx, cmd, observe)
		},
		CreateChild: func(ctx context.Context, root string, cmd remoteapp.ChildCreateCommand, observe remoteapp.AncestryObserver) (remoteapp.ChildCreateResult, error) {
			return newChildCreator(root).Create(ctx, cmd, observe)
		},
		VerifyRemoteArtifact:    verifyRemoteArtifact,
		ReflectRemoteCompletion: reflectRemoteCompletion,
		CloseRemoteIssue:        closeRemoteIssue,
		MergeRemotePullRequest:  mergeRemotePullRequest,

		CreateIssue:                  createIssue,
		ReadScoreSummaryFile:         remoteapp.NewTemplateBodyResolver(os.ReadFile).ScoreSummary,
		ReconcileIssueCreate:         reconcileIssueCreate,
		CreatePublication:            createPublication,
		IssueOpsStateRoot:            func() string { return root },
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		ReflectReviewFindings:        reflectReviewFindings,
		SyncRemoteBody:               syncRemoteBody,
		SyncIssueGraph:               syncIssueGraph,
	}}
}
