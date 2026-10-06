package operationalhealth

import (
	"testing"
	"time"
)

func TestParseTaskCompletedAtAcceptsRFC3339(t *testing.T) {
	for _, want := range []time.Time{
		time.Date(2026, 8, 9, 15, 24, 1, 123456789, time.FixedZone("KST", 9*60*60)),
		time.Date(2026, 8, 3, 22, 35, 17, 0, time.UTC),
	} {
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			raw := want.Format(layout)
			got, err := parseTaskCompletedAt("  " + raw + "  ")
			if err != nil {
				t.Fatalf("parseTaskCompletedAt(%q): %v", raw, err)
			}
			if !got.Equal(want.Truncate(time.Second)) && !got.Equal(want) {
				t.Fatalf("timestamp=%s want=%s", got, want)
			}
		}
	}
}

func TestParseTaskCompletedAtRejectsNonRFC3339(t *testing.T) {
	for _, raw := range []string{
		"2026-08-03 22:35:17",
		"2026/08/03 22:35:17",
		"2026-08-03 22:35:17Z",
		"2026-08-03T22:35:17",
		"not-a-timestamp",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := parseTaskCompletedAt(raw); err == nil {
				t.Fatalf("parseTaskCompletedAt(%q) unexpectedly succeeded", raw)
			}
		})
	}
}
