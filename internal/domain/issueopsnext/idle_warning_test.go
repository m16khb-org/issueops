package issueopsnext

import (
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestIdleWarning(t *testing.T) {
	now := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	stamp := func(ago time.Duration) string { return now.Add(-ago).Format(time.RFC3339Nano) }
	day := 24 * time.Hour

	for _, row := range []struct {
		name   string
		record issueopscontract.Record
		want   string // substring of the warning; "" means no warning
	}{
		{"6 days 23 hours ago", issueopscontract.Record{ID: "io-a", UpdatedAt: stamp(7*day - time.Hour)}, ""},
		{"exactly 7 days ago", issueopscontract.Record{ID: "io-a", UpdatedAt: stamp(7 * day)}, "7 days"},
		{"10 days ago", issueopscontract.Record{ID: "io-a", UpdatedAt: stamp(10 * day)}, "10 days"},
		{"all times empty", issueopscontract.Record{ID: "io-a"}, ""},
		{"garbage time", issueopscontract.Record{ID: "io-a", UpdatedAt: "garbage"}, ""},
		{"future time", issueopscontract.Record{ID: "io-a", UpdatedAt: stamp(-time.Hour)}, ""},
		{"lease claim newer than updated_at", issueopscontract.Record{
			ID: "io-a", UpdatedAt: stamp(10 * day),
			Execution: &issueopscontract.Execution{Lease: issueopscontract.WriteLease{ClaimedAt: stamp(day)}},
		}, ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			got := IdleWarning(row.record, now)
			if row.want == "" {
				if got != "" {
					t.Fatalf("IdleWarning = %q, want none", got)
				}
				return
			}
			last := row.record.UpdatedAt
			if !strings.Contains(got, row.want) || !strings.Contains(got, "last_record_at "+last) || !strings.Contains(got, "cycle io-a has had no record write for ") {
				t.Fatalf("IdleWarning = %q, want %q and last_record_at %s", got, row.want, last)
			}
			// next does not see stage or lease intent, so the warning only asks.
			if !strings.HasSuffix(got, "; ask the user whether it is still in progress") || strings.Contains(got, "abandon") || strings.Contains(got, "pause") {
				t.Fatalf("IdleWarning = %q, want a neutral ask-the-user clause", got)
			}
		})
	}
}
