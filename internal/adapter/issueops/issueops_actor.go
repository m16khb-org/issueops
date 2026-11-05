package issueops

import (
	"context"

	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
)

// validateExecutionMutation binds every durable IssueOps mutation to the
// current write lease once execution has been prepared. Planning mutations are
// intentionally actor-optional until the execution record exists.
func validateExecutionMutation(ctx context.Context, record issueops.Record, actor *issueops.Actor, verifier authorityport.ActorVerifier) error {
	return cycleapp.NewMutationAuthority(samePath, verifier).Validate(ctx, record, actor)
}

// ValidateIssueOpsMutationActor loads cycle id and checks that actor may mutate it under
// the current execution write lease. Records without a prepared execution accept any actor.
func ValidateIssueOpsMutationActor(ctx context.Context, stateRoot, id string, actor issueops.Actor, verifier authorityport.ActorVerifier) error {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return err
	}
	return validateExecutionMutation(ctx, record, &actor, verifier)
}
