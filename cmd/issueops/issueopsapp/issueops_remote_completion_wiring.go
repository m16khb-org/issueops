package issueopsapp

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops"
	"issueops/internal/adapter/provider"
	application "issueops/internal/application/issueopsremote"
	reportcontract "issueops/internal/contract/artifactreadability"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func newRemoteCompletionService(root string, resolve func(string) (port.IssueProvider, error), verify application.MergeVerifier, now func() time.Time) *application.RemoteCompletionService {
	store := issueops.RemoteRecordStore{StateRoot: root}
	return application.NewRemoteCompletionService(store, application.TrackedMaterials{Files: issueops.MaterialFiles{}}, application.NewCompletionReceipts(store, now), func(name string) (application.CompletionProvider, error) { return resolve(name) }, verify)
}

func reflectRemoteCompletion(ctx context.Context, root, id, providerOverride, resultBody string, confirm bool, verify application.MergeVerifier) (model.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, reportcontract.Report, error) {
	return newRemoteCompletionService(root, provider.Resolve, verify, time.Now).Reflect(ctx, id, providerOverride, resultBody, confirm)
}

func closeRemoteIssue(ctx context.Context, root, id, providerOverride string, confirm bool, verify application.MergeVerifier) (model.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
	return newRemoteCompletionService(root, provider.Resolve, verify, time.Now).Close(ctx, id, providerOverride, confirm)
}

func assignRemote(ctx context.Context, root, id, on string, users []string, confirm bool) (model.RemoteAssignResult, error) {
	return application.NewRemoteAssignService(issueops.RemoteRecordStore{StateRoot: root}, provider.ResolveAssigner).AddAssignees(ctx, id, on, users, confirm)
}

func retitleRemote(ctx context.Context, root, id, on, title string, confirm bool) (model.RemoteRetitleResult, error) {
	return application.NewRemoteRetitleService(issueops.RemoteRecordStore{StateRoot: root}, provider.ResolveRetitler).Retitle(ctx, id, on, title, confirm)
}

func labelRemote(ctx context.Context, root, id, on string, labels []string, confirm bool) (model.RemoteLabelResult, error) {
	return application.NewRemoteLabelService(issueops.RemoteRecordStore{StateRoot: root}, provider.ResolveLabeler).AddLabels(ctx, id, on, labels, confirm)
}

func commentRemote(ctx context.Context, root, id, on, body string, confirm bool) (model.RemoteCommentResult, error) {
	return application.NewRemoteCommentService(issueops.RemoteRecordStore{StateRoot: root}, provider.ResolveCommenter).Comment(ctx, id, on, body, confirm)
}

func markRemotePullRequestReady(ctx context.Context, root, id string, confirm bool) (model.RemoteReadyResult, error) {
	return application.NewRemoteMergeService(issueops.RemoteRecordStore{StateRoot: root}, provider.ResolvePullRequestMerger).MarkReady(ctx, id, confirm)
}

func mergeRemotePullRequest(ctx context.Context, root, id, method string, confirm bool) (model.RemoteMergeResult, error) {
	store := issueops.RemoteRecordStore{StateRoot: root}
	return application.NewRemoteMergeService(store, provider.ResolvePullRequestMerger).
		WithMergedReceipt(application.NewCompletionReceipts(store, time.Now).Merged).
		Merge(ctx, id, method, confirm)
}
