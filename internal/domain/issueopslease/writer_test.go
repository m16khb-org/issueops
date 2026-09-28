package issueopslease

import "testing"

func TestUnknownLeaseStatusStillHoldsWriter(t *testing.T) {
	for _, status := range []string{"active", "revoking", "", "future"} {
		if !LeaseHoldsWriter(status) {
			t.Fatalf("status %q failed open", status)
		}
	}
	for _, status := range []string{"claimable", "released"} {
		if LeaseHoldsWriter(status) {
			t.Fatalf("status %q blocked without writer", status)
		}
	}
}
