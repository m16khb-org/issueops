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

func newRemoteCompletionService(root string, resolve func(string) (port.IssueProvider, error), verify application.MergeVerifier, now func() time.Time) *application.RemoteCompletionService {
	store := issueops.RemoteRecordStore{StateRoot: root}
	return application.NewRemoteCompletionService(store, application.NewCompletionCollector(issueops.CompletionArtifacts{}), application.NewCompletionReceipts(store, now), func(name string) (application.CompletionProvider, error) { return resolve(name) }, verify)
}

func reflectRemoteCompletion(ctx context.Context, root, id, providerOverride string, confirm bool, verify application.MergeVerifier) (model.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	return newRemoteCompletionService(root, provider.Resolve, verify, time.Now).Reflect(ctx, id, providerOverride, confirm)
}

func closeRemoteIssue(ctx context.Context, root, id, providerOverride string, confirm bool, verify application.MergeVerifier) (model.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
	return newRemoteCompletionService(root, provider.Resolve, verify, time.Now).Close(ctx, id, providerOverride, confirm)
}
