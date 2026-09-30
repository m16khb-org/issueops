package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	artifactdomain "issueops/internal/domain/issueopsartifact"
	authorization "issueops/internal/domain/issueopsauthorization"
)

func (a MutationAuthority) ValidatePlanLink(record model.IssueOpsRecord, actor *model.IssueOpsActor) error {
	if record.Execution == nil || !artifactdomain.CanStage(record, "plan") {
		return a.Validate(record, actor)
	}
	canonical := actor != nil && a.pathsMatch != nil && a.pathsMatch(actor.CWD, record.Execution.Workspace.Root)
	return authorization.ValidatePlanCoordinator(actor, canonical)
}
