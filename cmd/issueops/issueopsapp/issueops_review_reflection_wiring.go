package issueopsapp

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	"issueops/internal/adapter/provider"
	cycleapp "issueops/internal/application/issueopscycle"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func newReviewReflectionService(root string, resolve func(string) (port.IssueProvider, error), observe application.AncestryObserver, now func() time.Time) *application.ReviewReflectionService {
	return application.NewReviewReflectionService(issueops.RemoteRecordStore{StateRoot: root}, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), func(name string) (application.ReviewReflectionProvider, error) { return resolve(name) }, observe, now)
}

func reflectReviewFindings(ctx context.Context, root, id, providerOverride string, confirm bool, actor model.IssueOpsActor, observe application.AncestryObserver) (model.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	return newReviewReflectionService(root, provider.Resolve, observe, time.Now).Reflect(ctx, id, providerOverride, confirm, actor)
}
