package jsonout

import (
	"bytes"
	"testing"
)

func TestPrintToWritesIndentedJSONWithTrailingNewline(t *testing.T) {
	var out bytes.Buffer
	if err := PrintTo(&out, map[string]any{"ok": true, "items": []int{1}}); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"items\": [\n    1\n  ],\n  \"ok\": true\n}\n"
	if out.String() != want {
		t.Fatalf("output=%q, want %q", out.String(), want)
	}
}
