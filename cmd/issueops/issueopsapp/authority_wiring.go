package issueopsapp

import (
	"context"
	"crypto/rand"
	"time"

	issueopscore "issueops/internal/adapter/issueops"
	authorityoutbound "issueops/internal/adapter/outbound/authority"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	authorityapp "issueops/internal/application/authority"
	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
)

// newAuthorityService composes immutable dependencies only. Grants live in the
// fixed user-state IssueOps database; request identity stays in the context.
func newAuthorityService() *authorityapp.Service {
	return authorityapp.New(
		authorityoutbound.Repository{StateRoot: issueOpsStateRoot()},
		issueopscore.NativeProcessInspector{},
		authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()},
		time.Now,
		authorityoutbound.ScopeResolver{},
		rand.Reader,
	)
}

func issueOpsActorVerifier() authorityport.ActorVerifier { return newAuthorityService() }

// bindIssueOpsAuthority binds a request credential and arms the sqlstore record
// guard on the grant root, so each span and data write of the request on that
// root rechecks the grant under its lock. Spans on other roots (the reseed
// fence, loop, or worker stores) pass the guard through.
func bindIssueOpsAuthority(ctx context.Context, use authoritycontract.Use) (context.Context, model.VerifiedActor, error) {
	service := newAuthorityService()
	bound, verified, err := service.Bind(ctx, use)
	if err != nil {
		return ctx, model.VerifiedActor{}, err
	}
	return sqlstore.WithRecordGuard(bound, issueOpsStateRoot(), service.BindSpan), verified, nil
}
