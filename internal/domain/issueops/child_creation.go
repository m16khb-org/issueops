package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"strings"
)

func ValidateChildCreation(record model.IssueOpsRecord, title string, labels, assignees []string) error {
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot create child before linked parent issue")
	}
	if reason := UmbrellaBranchGateReason(record); reason != "" {
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

func ValidateCreatedChild(hierarchyVerified bool, childURL string) error {
	if !hierarchyVerified || strings.TrimSpace(childURL) == "" {
		return fmt.Errorf("provider did not verify child hierarchy")
	}
	return nil
}
