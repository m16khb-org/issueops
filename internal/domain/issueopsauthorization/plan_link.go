package issueopsauthorization

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueopsauthorization"
)

func ValidatePlanCoordinator(actor *model.Actor, canonicalWorkspace bool) error {
	host := ""
	if actor != nil {
		host = strings.ToLower(strings.TrimSpace(actor.Host))
	}
	if actor == nil || (host != "codex" && host != "claude" && host != "omo") || strings.TrimSpace(actor.SessionID) == "" || len(actor.NativeProcessAncestry) == 0 || !canonicalWorkspace {
		return fmt.Errorf("released Orca plan linking requires a native coordinator in the canonical worktree")
	}
	return nil
}
