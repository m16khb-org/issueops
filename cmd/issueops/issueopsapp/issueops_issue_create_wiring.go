package issueopsapp

import (
	"context"
	"os"
	"time"

	"issueops/internal/adapter/issueops"
	"issueops/internal/adapter/provider"
	application "issueops/internal/application/issueopsremote"
	"issueops/internal/port"
)

func newIssueCreator(root string, resolve func(string) (port.IssueProvider, error), verify application.IssueLiveVerifier, now func() time.Time) *application.IssueCreator {
	store := issueops.IssueCreateIntentStore{StateRoot: root}
	return application.NewIssueCreator(store, issueops.IssueCreationEnvironment{ResolveProvider: resolve}, application.NewTemplateBodyResolver(os.ReadFile), application.NewIssueCreateIntents(store, now), verify, now)
}
func createIssue(ctx context.Context, root string, cmd application.IssueCreateCommand, verify application.IssueLiveVerifier) (port.IssueProviderCreateIssueResult, error) {
	return newIssueCreator(root, provider.Resolve, verify, time.Now).Create(ctx, cmd)
}
