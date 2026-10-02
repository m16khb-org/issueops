package issueopsauthorization

import (
	"fmt"
	"strings"

	issueopsauthorizationcontract "issueops/internal/contract/issueopsauthorization"
)

// ValidateHolder returns whether the application must observe canonical path identity.
// verified is the caller identity proven by the application verifier (native
// ancestry or a bound capability); its ancestry is never required here.
func ValidateHolder(record issueopsauthorizationcontract.Record, actor *issueopsauthorizationcontract.Actor, verified *issueopsauthorizationcontract.VerifiedActor) (bool, error) {
	if record.Execution == nil {
		return false, nil
	}
	lease := record.Execution.Lease
	if lease.Status != issueopsauthorizationcontract.LeaseStatusActive || lease.Holder == nil {
		return false, fmt.Errorf(
			"IssueOps execution generation %d has no active write lease",
			lease.Generation,
		)
	}
	holderRequired := fmt.Errorf("IssueOps execution mutation requires the current write lease holder")
	if actor == nil ||
		!strings.EqualFold(strings.TrimSpace(actor.Host), lease.Holder.Host) ||
		strings.TrimSpace(actor.SessionID) != lease.Holder.SessionID ||
		strings.TrimSpace(actor.AgentID) != lease.Holder.AgentID {
		return false, holderRequired
	}
	if verified == nil || lease.Holder.SessionProcess == nil || verified.Identity.SessionProcess == nil ||
		!strings.EqualFold(verified.Identity.Host, lease.Holder.Host) ||
		verified.Identity.SessionID != lease.Holder.SessionID ||
		verified.Identity.AgentID != lease.Holder.AgentID ||
		*verified.Identity.SessionProcess != *lease.Holder.SessionProcess {
		return false, holderRequired
	}
	return true, nil
}

func ValidateCWD(canonical bool) error {
	if !canonical {
		return fmt.Errorf("IssueOps execution mutation requires the canonical worktree cwd")
	}
	return nil
}
