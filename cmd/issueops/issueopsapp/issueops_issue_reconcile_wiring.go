package issueopsapp

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops"
	"issueops/internal/adapter/provider"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func newIssueReconciler(root string, resolve func(string) (port.IssueProvider, error), verify application.IssueLiveVerifier, now func() time.Time) *application.IssueReconciler {
	store := issueops.RemoteRecordStore{StateRoot: root}
	return application.NewIssueReconciler(store, issueops.IssueCreateCandidateSource{Resolve: resolve}, application.NewIssueCreateIntents(store, now), verify, now)
}

func reconcileIssueCreate(ctx context.Context, root, id string, confirm bool, verify application.IssueLiveVerifier) (model.IssueOpsIssueCreateReconcileResult, error) {
	return newIssueReconciler(root, provider.Resolve, verify, time.Now).Reconcile(ctx, id, confirm)
}
