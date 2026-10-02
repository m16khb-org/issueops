package issueopsapp

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	cycleapp "issueops/internal/application/issueopscycle"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

func newArtifactVerificationService(root string, verify application.ArtifactLiveVerifier, observe application.AncestryObserver, now func() time.Time) *application.ArtifactVerificationService {
	return application.NewArtifactVerificationService(issueops.RemoteRecordStore{StateRoot: root}, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueOpsActorVerifier()), verify, observe, now)
}

func verifyRemoteArtifact(ctx context.Context, root, id string, req model.IssueOpsRemoteArtifactVerificationRequest, actor model.IssueOpsActor, verify application.ArtifactLiveVerifier, observe application.AncestryObserver) (model.IssueOpsRecord, error) {
	return newArtifactVerificationService(root, verify, observe, time.Now).Verify(ctx, id, req, actor)
}
