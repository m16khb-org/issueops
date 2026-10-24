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
		// QA agents had to find the way back to a lease in the skill docs. A
		// released generation (completed or not) is reclaimed through the
		// replacement chain; other states are routed by next.
		remedy := "run `issueops next --id " + record.ID + " --json` and follow its next_command to take the lease before writing"
		if lease.Status == issueopsauthorizationcontract.LeaseStatusReleased {
			remedy = fmt.Sprintf("to write again, take a new generation: `issueops execution replace --id %s --expected-generation %d --preview --json`, then the --reseed and claim commands it returns", record.ID, lease.Generation)
		}
		return false, fmt.Errorf("IssueOps execution generation %d has no active write lease (status %s); %s", lease.Generation, lease.Status, remedy)
	}
	// The refusal names the first mismatch and the recovery step; holder
	// identity is already public in `status`.
	holderRequired := func(detail string) error {
		return fmt.Errorf("IssueOps execution mutation requires the current write lease holder: %s; run `issueops execution whoami --json` in the holder session and pass its claim flags", detail)
	}
	switch {
	case actor == nil || strings.TrimSpace(actor.Host) == "" && strings.TrimSpace(actor.SessionID) == "":
		return false, holderRequired("no actor flags were passed")
	case !strings.EqualFold(strings.TrimSpace(actor.Host), lease.Holder.Host):
		return false, holderRequired(fmt.Sprintf("caller host %q, holder host %q", strings.TrimSpace(actor.Host), lease.Holder.Host))
	case strings.TrimSpace(actor.SessionID) != lease.Holder.SessionID:
		return false, holderRequired(fmt.Sprintf("caller session id %q, holder session id %q", strings.TrimSpace(actor.SessionID), lease.Holder.SessionID))
	case strings.TrimSpace(actor.AgentID) != lease.Holder.AgentID:
		return false, holderRequired(fmt.Sprintf("caller agent id %q, holder agent id %q", strings.TrimSpace(actor.AgentID), lease.Holder.AgentID))
	}
	if verified == nil || lease.Holder.SessionProcess == nil || verified.Identity.SessionProcess == nil ||
		!strings.EqualFold(verified.Identity.Host, lease.Holder.Host) ||
		verified.Identity.SessionID != lease.Holder.SessionID ||
		verified.Identity.AgentID != lease.Holder.AgentID ||
		*verified.Identity.SessionProcess != *lease.Holder.SessionProcess {
		return false, holderRequired("the caller's verified session process is not the holder's")
	}
	return true, nil
}

// ValidateCWD refuses a caller outside the canonical worktree and names both
// paths, so the caller can correct --cwd without reading the record.
func ValidateCWD(canonical bool, cwd, root string) error {
	if !canonical {
		return fmt.Errorf("IssueOps execution mutation requires the canonical worktree cwd: rerun with --cwd %q (got %q)", root, cwd)
	}
	return nil
}
