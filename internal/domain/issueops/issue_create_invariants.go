package issueops

import (
	"fmt"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

func ValidateIssueCreateIntentInvariants(intent issueopscontract.IssueOpsIssueCreateIntent) error {
	if intent.Status == issueopscontract.IssueCreateIntentCompleted && strings.TrimSpace(intent.CanonicalURL) == "" {
		return fmt.Errorf("completed issue create intent requires canonical_url")
	}
	if (intent.Status == issueopscontract.IssueCreateIntentPending ||
		intent.Status == issueopscontract.IssueCreateIntentNotInvoked ||
		intent.Status == issueopscontract.IssueCreateIntentInvokedUnknown) &&
		strings.TrimSpace(intent.CanonicalURL) != "" {
		return fmt.Errorf("issue create intent status %s must not have canonical_url", intent.Status)
	}
	if intent.Status != issueopscontract.IssueCreateIntentPending &&
		intent.Status != issueopscontract.IssueCreateIntentCompleted &&
		strings.TrimSpace(intent.Failure) == "" {
		return fmt.Errorf("issue create intent status %s requires failure", intent.Status)
	}
	return nil
}
