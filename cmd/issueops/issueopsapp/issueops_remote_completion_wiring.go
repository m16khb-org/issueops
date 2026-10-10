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

func mergeRemotePullRequest(ctx context.Context, root, id, method string, confirm bool) (model.RemoteMergeResult, error) {
	return application.NewRemoteMergeService(issueops.RemoteRecordStore{StateRoot: root}, provider.ResolvePullRequestMerger).Merge(ctx, id, method, confirm)
}
