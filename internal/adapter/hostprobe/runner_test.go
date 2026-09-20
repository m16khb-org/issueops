package hostprobe

import (
	"strings"
	"testing"
)

func TestObservedModelFromOutputReadsOnlyStructuredModelFields(t *testing.T) {
	output := []byte("{\"type\":\"system\",\"subtype\":\"init\",\"model\":\"claude-opus-5\"}\n{\"model\":\"later\"}\n")
	if got := observedModelFromOutput(output); got != "claude-opus-5" {
		t.Fatalf("observed model=%q", got)
	}
	if got := observedModelFromOutput([]byte(`{"text":"model=secret"}`)); got != "" {
		t.Fatalf("freeform text produced model=%q", got)
	}
}

func TestBoundedBufferReportsTruncationWithoutShortWrite(t *testing.T) {
	buffer := &boundedBuffer{limit: 4}
	value := []byte("123456")
	written, err := buffer.Write(value)
	if err != nil || written != len(value) || buffer.String() != "1234" || !buffer.truncated {
		t.Fatalf("written=%d err=%v buffer=%q truncated=%t", written, err, buffer.String(), buffer.truncated)
	}
	if strings.Contains(buffer.String(), "56") {
		t.Fatal("bounded buffer retained truncated suffix")
	}
}

func TestSemanticResponseDigestIsHostNeutral(t *testing.T) {
	shapes := []any{
		map[string]any{"content": "captured"},
		map[string]any{"content": "captured", "duration_ms": 17, "server": "issueops_probe"},
		map[string]any{
			"content": []any{map[string]any{"type": "text", "text": "captured"}},
			"details": map[string]any{"server": "issueops_probe", "tool": "harness_probe_empty_object"},
		},
	}
	var want string
	for index, shape := range shapes {
		got, err := semanticResponseDigest([]any{shape})
		if err != nil {
			t.Fatalf("shape %d: %v", index, err)
		}
		if index == 0 {
			want = got
		} else if got != want {
			t.Fatalf("shape %d digest = %q, want %q", index, got, want)
		}
	}
	different, err := semanticResponseDigest([]any{map[string]any{"content": "different"}})
	if err != nil {
		t.Fatal(err)
	}
	if different == want {
		t.Fatal("semantic response difference produced the same digest")
	}
}
