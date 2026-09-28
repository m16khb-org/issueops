package issueops

import (
	"fmt"
	"strings"

	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	issueopsartifactdomain "issueops/internal/domain/issueopsartifact"
)

// validateExecutionMutation binds every durable IssueOps mutation to the
// current write lease once execution has been prepared. Planning mutations are
// intentionally actor-optional until the execution record exists.
func validateExecutionMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	return cycleapp.NewMutationAuthority(samePath).Validate(record, actor)
}

func validatePlanLinkMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	if record.Execution == nil || !issueopsartifactdomain.CanStage(record, "plan") {
		return validateExecutionMutation(record, actor)
	}
	host := ""
	if actor != nil {
		host = strings.ToLower(strings.TrimSpace(actor.Host))
	}
	if actor == nil || (host != "codex" && host != "claude" && host != "omo") ||
		strings.TrimSpace(actor.SessionID) == "" || len(actor.NativeProcessAncestry) == 0 ||
		!samePath(actor.CWD, record.Execution.Workspace.Root) {
		return fmt.Errorf("released Orca plan linking requires a native coordinator in the canonical worktree")
	}
	return nil
}

func validateWorkspacePreparationMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	return validateExecutionMutation(record, actor)
}

func ValidateIssueOpsMutationActor(stateRoot, id string, actor IssueOpsActor) error {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return err
	}
	return validateWorkspacePreparationMutation(record, &actor)
}

// validatePostTransferMutation keeps current-contract durable writes bound to the
// owner even when callers bypass lifecycle hooks through a direct CLI or MCP
// request. Legacy cycles retain their existing actor-optional behavior.
func validatePostTransferMutation(record issueops.IssueOpsRecord, actor *IssueOpsActor) error {
	return validateExecutionMutation(record, actor)
}
