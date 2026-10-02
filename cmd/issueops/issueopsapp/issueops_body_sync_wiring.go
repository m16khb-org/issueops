package issueopsapp

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	application "issueops/internal/application/issueopsbodysync"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/port"
)

func syncIssueOpsRemoteArtifactBody(ctx context.Context, stateRoot, id string, command contract.Command, provider port.IssueProvider, actor model.IssueOpsActor) (model.IssueOpsRecord, contract.Result, error) {
	gateway, err := issueops.NewBodySyncProvider(provider)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, contract.Result{}, err
	}
	service := application.NewService(issueops.BodySyncRepository{StateRoot: stateRoot}, gateway, cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueOpsActorVerifier()), time.Now)
	return service.Sync(ctx, id, command, actor)
}
