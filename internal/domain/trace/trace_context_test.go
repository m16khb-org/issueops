package trace

import (
	"strings"
	"testing"
)

func TestParseTraceparent(t *testing.T) {
	const valid = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	for _, tc := range []struct {
		name, input string
		valid       bool
	}{
		{"valid", valid, true},
		{"reserved flags", valid[:53] + "ff", true},
		{"empty", "", false},
		{"short", valid[:54], false},
		{"extra field", valid + "-01", false},
		{"unsupported version", "01" + valid[2:], false},
		{"uppercase", strings.ToUpper(valid), false},
		{"zero trace", "00-" + strings.Repeat("0", 32) + valid[35:], false},
		{"zero parent", valid[:36] + strings.Repeat("0", 16) + valid[52:], false},
		{"invalid flags", valid[:53] + "gg", false},
		{"invalid separator", valid[:35] + "_" + valid[36:], false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseTraceparent(tc.input)
			if ok != tc.valid {
				t.Fatalf("valid=%v, want %v", ok, tc.valid)
			}
			if !ok {
				if got.TraceID != "" || got.ParentID != "" || got.Flags != "" {
					t.Fatalf("invalid input retained correlation values: %+v", got)
				}
				return
			}
			if got.TraceID != tc.input[3:35] || got.ParentID != tc.input[36:52] || got.Flags != tc.input[53:55] {
				t.Fatalf("wrong correlation fields: %+v", got)
			}
		})
	}
}
