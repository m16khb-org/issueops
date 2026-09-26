package operationalhealth

import (
	"testing"
	"time"
)

func TestIssueCreateIntentNeedsReconciliationAtBoundary(t *testing.T) {
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-5 * time.Minute).Format(time.RFC3339Nano)
	if IssueCreateIntentNeedsReconciliationAt("pending", updatedAt, "", now) {
		t.Fatal("exact threshold must remain pending")
	}
	updatedAt = now.Add(-5*time.Minute - time.Nanosecond).Format(time.RFC3339Nano)
	if !IssueCreateIntentNeedsReconciliationAt("pending", updatedAt, "", now) {
		t.Fatal("older pending intent needs reconciliation")
	}
	if !IssueCreateIntentNeedsReconciliationAt("pending", "invalid", "", now) {
		t.Fatal("unparseable time needs reconciliation")
	}
}
