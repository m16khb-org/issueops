package issueopsauthorization

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueopsauthorization"
)

// ValidatePlanCoordinator accepts a native coordinator proven either by its
// observed ancestry or by a verified capability identity matching the actor.
func ValidatePlanCoordinator(actor *model.Actor, verified *model.VerifiedActor, canonicalWorkspace bool) error {
	host := ""
	if actor != nil {
		host = strings.ToLower(strings.TrimSpace(actor.Host))
	}
	if actor == nil || (host != "codex" && host != "claude" && host != "omo" && host != "omp") || strings.TrimSpace(actor.SessionID) == "" ||
		!(len(actor.NativeProcessAncestry) > 0 || verifiedCoordinator(actor, host, verified)) || !canonicalWorkspace {
		return fmt.Errorf("released Orca plan linking requires a native coordinator in the canonical worktree")
	}
	return nil
}

func verifiedCoordinator(actor *model.Actor, host string, verified *model.VerifiedActor) bool {
	return verified != nil && strings.EqualFold(verified.Identity.Host, host) &&
		verified.Identity.SessionID == strings.TrimSpace(actor.SessionID) &&
		verified.Identity.AgentID == strings.TrimSpace(actor.AgentID)
}
