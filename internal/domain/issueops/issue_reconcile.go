package issueops

import (
	"crypto/sha256"
	"fmt"

	model "issueops/internal/contract/issueops"
)

func ValidateIssueReconcileIntent(intent *model.IssueOpsIssueCreateIntent) error {
	if intent == nil {
		return fmt.Errorf("no issue create intent to reconcile")
	}
	if intent.Status == model.IssueCreateIntentCompleted {
		return fmt.Errorf("issue create intent is already completed")
	}
	return nil
}

func ValidateIssueReconcileSearch(truncated bool, count int) error {
	if truncated {
		return fmt.Errorf("issue create reconciliation search was truncated; uniqueness is indeterminate")
	}
	if count != 1 {
		return fmt.Errorf("issue create reconciliation found %d live candidates; exactly one is required", count)
	}
	return nil
}

func ValidateIssueReconcileCandidate(intent model.IssueOpsIssueCreateIntent, authority, title, body string) error {
	if authority == "" || authority != intent.ProjectAuthority {
		return fmt.Errorf("issue create reconciliation candidate authority %q does not match sealed authority %q", authority, intent.ProjectAuthority)
	}
	digest := sha256.Sum256([]byte(body))
	if title != intent.Title || fmt.Sprintf("%x", digest[:]) != intent.BodySHA256 {
		return fmt.Errorf("issue create reconciliation candidate does not match sealed title and body digest")
	}
	return nil
}
