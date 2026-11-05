package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"strings"
)

// ValidateChildCreation checks the preconditions for creating a child issue:
// the parent is linked, the umbrella branch gate passes, and a title, at
// least one label, and at least one assignee are given.
func ValidateChildCreation(record model.Record, title string, labels, assignees []string) error {
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot create child before linked parent issue")
	}
	if reason := umbrellaBranchGateReason(record); reason != "" {
		return fmt.Errorf("%s", reason)
	}
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("child title is required")
	}
	if len(labels) == 0 {
		return fmt.Errorf("at least one child label is required")
	}
	if len(assignees) == 0 {
		return fmt.Errorf("at least one child assignee is required")
	}
	return nil
}

// ValidateCreatedChild fails unless the provider verified the child hierarchy
// and returned a non-blank child URL.
func ValidateCreatedChild(hierarchyVerified bool, childURL string) error {
	if !hierarchyVerified || strings.TrimSpace(childURL) == "" {
		return fmt.Errorf("provider did not verify child hierarchy")
	}
	return nil
}
