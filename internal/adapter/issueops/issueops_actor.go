package issueops

import (
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
)

// validateExecutionMutation binds every durable IssueOps mutation to the
// current write lease once execution has been prepared. Planning mutations are
// intentionally actor-optional until the execution record exists.
func validateExecutionMutation(record issueops.IssueOpsRecord, actor *issueops.IssueOpsActor) error {
	return cycleapp.NewMutationAuthority(samePath).Validate(record, actor)
}

func validateWorkspacePreparationMutation(record issueops.IssueOpsRecord, actor *issueops.IssueOpsActor) error {
	return validateExecutionMutation(record, actor)
}

func ValidateIssueOpsMutationActor(stateRoot, id string, actor issueops.IssueOpsActor) error {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return err
	}
	return validateWorkspacePreparationMutation(record, &actor)
}

// validatePostTransferMutation keeps current-contract durable writes bound to the
// owner even when callers bypass lifecycle hooks through a direct CLI or MCP
// request. Legacy cycles retain their existing actor-optional behavior.
func validatePostTransferMutation(record issueops.IssueOpsRecord, actor *issueops.IssueOpsActor) error {
	return validateExecutionMutation(record, actor)
}
