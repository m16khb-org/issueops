package operationalhealth

import (
	"strings"
	"time"
)

const staleIssueCreatePendingAfter = 5 * time.Minute

func IssueCreateIntentNeedsReconciliationAt(status, updatedAt, startedAt string, now time.Time) bool {
	switch status {
	case "pending":
		updatedAt = strings.TrimSpace(updatedAt)
		if updatedAt == "" {
			updatedAt = strings.TrimSpace(startedAt)
		}
		observedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return true
		}
		return now.UTC().Sub(observedAt.UTC()) > staleIssueCreatePendingAfter
	case "invoked_unknown", "url_observed", "verification_failed", "receipt_failed":
		return true
	default:
		return false
	}
}
