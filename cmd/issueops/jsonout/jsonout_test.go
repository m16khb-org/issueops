package jsonout

import (
	"bytes"
	"errors"
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

// A failed result keeps its fields, gains the error once, and never loses
// integer precision on the way through the map.
func TestPrintResultErrorAddsErrorOnceAndKeepsFields(t *testing.T) {
	type result struct {
		OK    bool   `json:"ok"`
		ID    int64  `json:"id"`
		Error string `json:"error,omitempty"`
	}
	boom := errors.New("boom")
	var out bytes.Buffer
	err := PrintResultError(&out, result{ID: 9007199254740993}, boom)
	if !errors.Is(err, boom) || !IsReported(err) {
		t.Fatalf("err=%v, want boom marked reported", err)
	}
	if want := "{\n  \"error\": \"boom\",\n  \"id\": 9007199254740993,\n  \"ok\": false\n}\n"; out.String() != want {
		t.Fatalf("output=%q, want %q", out.String(), want)
	}
	if got := WithError(result{Error: "own"}, boom)["error"]; got != "own" {
		t.Fatalf("existing error replaced: %v", got)
	}
	if got := WithError([]int{1}, boom); got["ok"] != false || got["error"] != "boom" {
		t.Fatalf("non-object result = %v, want ok:false with the error", got)
	}
}
