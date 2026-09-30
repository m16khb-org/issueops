package daemon

import (
	"testing"
	"time"
)

func TestDaemonSettingsPreserveBoundsAndFallbacks(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int
	}{{"", 256}, {"1", 1}, {"4096", 4096}, {"4097", 256}, {"0", 256}, {"-1", 256}, {" 7 ", 256}, {"99999999999999999999999", 256}} {
		if got := MaxConnections(tc.input); got != tc.want {
			t.Fatalf("capacity %q: %d, want %d", tc.input, got, tc.want)
		}
	}
	for _, tc := range []struct {
		input string
		want  time.Duration
	}{{"", 30 * time.Minute}, {"invalid", 30 * time.Minute}, {"0s", 30 * time.Minute}, {"-1s", 30 * time.Minute}, {"1ns", time.Nanosecond}, {"2m", 2 * time.Minute}} {
		if got := IdleTimeout(tc.input); got != tc.want {
			t.Fatalf("idle %q: %v, want %v", tc.input, got, tc.want)
		}
	}
}
