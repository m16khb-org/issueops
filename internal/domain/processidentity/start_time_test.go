package processidentity

import (
	"testing"
	"time"
)

func TestCanonicalStartTimeNormalizesSupportedObservations(t *testing.T) {
	got, err := CanonicalStartTime("Fri Jul 31 16:14:57 2026", time.UTC)
	if err != nil || got != "2026-07-31T16:14:57Z" {
		t.Fatalf("C-locale lstart: got=%q err=%v", got, err)
	}
	got, err = CanonicalStartTime("2026-07-31T07:14:57.50Z", time.UTC)
	if err != nil || got != "2026-07-31T07:14:57.5Z" {
		t.Fatalf("RFC3339 fractional: got=%q err=%v", got, err)
	}
	if got, err := CanonicalStartTime("linux:123:100", time.UTC); err != nil || got != "linux:123:100" {
		t.Fatalf("linux receipt must pass through: got=%q err=%v", got, err)
	}
	for _, invalid := range []string{"", "  ", "2026년 7월 31일 금요일 16시 14분 57초"} {
		if _, err := CanonicalStartTime(invalid, time.UTC); err == nil {
			t.Fatalf("expected %q to be rejected", invalid)
		}
	}
}
