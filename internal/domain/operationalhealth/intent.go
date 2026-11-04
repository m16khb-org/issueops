package operationalhealth

import (
	"strings"
	"time"
)

const staleIssueCreatePendingAfter = 5 * time.Minute

// IssueCreateIntentNeedsReconciliationAt 함수는 issue 생성 의도가 조정(reconcile)이
// 필요한 상태인지 판단한다. pending은 updatedAt(없으면 startedAt)이 now보다 5분
// 넘게 지났거나 시각을 해석할 수 없을 때, invoked_unknown, url_observed,
// verification_failed, receipt_failed는 항상 조정이 필요하다.
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
