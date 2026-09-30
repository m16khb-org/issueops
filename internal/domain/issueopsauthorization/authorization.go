package issueopsauthorization

import (
	"fmt"
	"strings"

	issueopsauthorizationcontract "issueops/internal/contract/issueopsauthorization"
)

// ValidateHolder returns whether the application must observe canonical path identity.
func ValidateHolder(record issueopsauthorizationcontract.Record, actor *issueopsauthorizationcontract.Actor) (bool, error) {
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
	if actor == nil ||
		!strings.EqualFold(strings.TrimSpace(actor.Host), lease.Holder.Host) ||
		strings.TrimSpace(actor.SessionID) != lease.Holder.SessionID ||
		strings.TrimSpace(actor.AgentID) != lease.Holder.AgentID {
		return false, fmt.Errorf("IssueOps execution mutation requires the current write lease holder")
	}
	processMatches := false
	for _, observed := range actor.NativeProcessAncestry {
		if lease.Holder.SessionProcess != nil && observed == *lease.Holder.SessionProcess {
			processMatches = true
			break
		}
	}
	if !processMatches {
		return false, fmt.Errorf("IssueOps execution mutation requires the current write lease holder")
	}
	return true, nil
}

func ValidateCWD(canonical bool) error {
	if !canonical {
		return fmt.Errorf("IssueOps execution mutation requires the canonical worktree cwd")
	}
	return nil
}
