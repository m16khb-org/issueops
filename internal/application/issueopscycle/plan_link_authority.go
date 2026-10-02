package issueopscycle

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	artifactdomain "issueops/internal/domain/issueopsartifact"
	authorization "issueops/internal/domain/issueopsauthorization"
)

func (a MutationAuthority) ValidatePlanLink(ctx context.Context, record model.IssueOpsRecord, actor *model.IssueOpsActor) error {
	if record.Execution == nil || !artifactdomain.CanStage(record, "plan") {
		return a.Validate(ctx, record, actor)
	}
	canonical := actor != nil && a.pathsMatch != nil && a.pathsMatch(actor.CWD, record.Execution.Workspace.Root)
	var verified *model.VerifiedActor
	if actor != nil && len(actor.NativeProcessAncestry) == 0 && a.verifier != nil {
		identity, err := a.verifier.Verify(ctx, model.NativeActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID})
		if err != nil {
			return fmt.Errorf("released Orca plan linking requires a native coordinator in the canonical worktree: %w", err)
		}
		verified = &identity
	}
	return authorization.ValidatePlanCoordinator(actor, verified, canonical)
}
