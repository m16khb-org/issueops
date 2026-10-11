package policy

import (
	"strings"
	"testing"
)

func TestBoundedDiagnosticUTF8Safe(t *testing.T) {
	if got, want := BoundedDiagnostic(strings.Repeat("가", 10), 16), "가가가가가...[truncated]"; got != want {
		t.Fatalf("BoundedDiagnostic = %q, want %q", got, want)
	}
}

func TestBoundedDiagnosticKeepsValueAtOrUnderLimit(t *testing.T) {
	cases := []struct {
		name  string
		value string
		limit int
		want  string
	}{
		{"exactly at limit", "abcde", 5, "abcde"},
		{"one over limit", "abcdef", 5, "abcde...[truncated]"},
		{"zero limit uses default so short value is kept", "abc", 0, "abc"},
		{"negative limit uses default so short value is kept", "abc", -1, "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BoundedDiagnostic(tc.value, tc.limit); got != tc.want {
				t.Fatalf("BoundedDiagnostic(%q, %d) = %q, want %q", tc.value, tc.limit, got, tc.want)
			}
		})
	}
}
