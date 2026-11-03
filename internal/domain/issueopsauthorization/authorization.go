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
		return false, noActiveLeaseError(record)
	}
	if detail := actorMismatch(actor, record); detail != "" {
		return false, holderRequiredError(detail)
	}
	if !verifiedMatchesHolder(verified, record) {
		return false, holderRequiredError("the caller's verified session process is not the holder's")
	}
	return true, nil
}

func noActiveLeaseError(record issueopsauthorizationcontract.Record) error {
	id, lease := record.ID, record.Execution.Lease
	// QA agents had to find the way back to a lease in the skill docs. A
	// released generation (completed or not) is reclaimed through the
	// replacement chain; other states are routed by next.
	remedy := "run `issueops next --id " + id + " --json` and follow its next_command to take the lease before writing"
	if lease.Status == issueopsauthorizationcontract.LeaseStatusReleased {
		remedy = fmt.Sprintf("to write again, take a new generation: `issueops execution replace --id %s --expected-generation %d --preview --json`, then the --reseed and claim commands it returns", id, lease.Generation)
	}
	return fmt.Errorf("IssueOps execution generation %d has no active write lease (status %s); %s", lease.Generation, lease.Status, remedy)
}

// holderRequiredError refuses naming the first mismatch and the recovery step;
// holder identity is already public in `status`.
func holderRequiredError(detail string) error {
	return fmt.Errorf("IssueOps execution mutation requires the current write lease holder: %s; run `issueops execution whoami --json` in the holder session and pass its claim flags", detail)
}

// actorMismatch returns the first difference between the caller's claim flags
// and the record's lease holder, or "" when they agree. The holder is non-nil.
func actorMismatch(actor *issueopsauthorizationcontract.Actor, record issueopsauthorizationcontract.Record) string {
	holder := record.Execution.Lease.Holder
	switch {
	case actor == nil || strings.TrimSpace(actor.Host) == "" && strings.TrimSpace(actor.SessionID) == "":
		return "no actor flags were passed"
	case !strings.EqualFold(strings.TrimSpace(actor.Host), holder.Host):
		return fmt.Sprintf("caller host %q, holder host %q", strings.TrimSpace(actor.Host), holder.Host)
	case strings.TrimSpace(actor.SessionID) != holder.SessionID:
		return fmt.Sprintf("caller session id %q, holder session id %q", strings.TrimSpace(actor.SessionID), holder.SessionID)
	case strings.TrimSpace(actor.AgentID) != holder.AgentID:
		return fmt.Sprintf("caller agent id %q, holder agent id %q", strings.TrimSpace(actor.AgentID), holder.AgentID)
	}
	return ""
}

// verifiedMatchesHolder reports whether the verified identity, including its
// session process, is exactly the lease holder's.
func verifiedMatchesHolder(verified *issueopsauthorizationcontract.VerifiedActor, record issueopsauthorizationcontract.Record) bool {
	holder := record.Execution.Lease.Holder
	if verified == nil || holder.SessionProcess == nil || verified.Identity.SessionProcess == nil {
		return false
	}
	return strings.EqualFold(verified.Identity.Host, holder.Host) &&
		verified.Identity.SessionID == holder.SessionID &&
		verified.Identity.AgentID == holder.AgentID &&
		*verified.Identity.SessionProcess == *holder.SessionProcess
}

// ValidateCWD refuses a caller outside the canonical worktree and names both
// paths, so the caller can correct --cwd without reading the record.
func ValidateCWD(canonical bool, cwd, root string) error {
	if !canonical {
		return fmt.Errorf("IssueOps execution mutation requires the canonical worktree cwd: rerun with --cwd %q (got %q)", root, cwd)
	}
	return nil
}
