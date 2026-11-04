package issueops

import model "issueops/internal/contract/issueops"

// ExecutionReadinessMissing lists the execution gates not met: a recorded
// execution, a valid execution record, a workspace that matches the current
// directory (workspaceMatches, observed by the caller), and an active write
// lease with a holder.
func ExecutionReadinessMissing(record model.Record, workspaceMatches bool) []string {
	if record.Execution == nil {
		return []string{"execution"}
	}
	missing := []string{}
	if err := ValidateExecution(*record.Execution); err != nil {
		missing = append(missing, "execution_valid")
	}
	if !workspaceMatches {
		missing = append(missing, "execution_worktree_match")
	}
	if record.Execution.Lease.Status != model.LeaseStatusActive || record.Execution.Lease.Holder == nil {
		missing = append(missing, "execution_write_lease")
	}
	return missing
}
