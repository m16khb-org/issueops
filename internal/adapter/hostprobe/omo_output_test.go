package hostprobe

import (
	"strings"
	"testing"
)

func TestOmoOutputKeepsRequiredAndInvalidEventsAcrossChunks(t *testing.T) {
	for _, suffix := range []string{
		`{"type":"tool_execution_start","toolName":"unexpected"}`,
		`{"type":"message_update",broken}`,
		`{"type":"unknown_event"}`,
	} {
		output := &boundedBuffer{limit: MaxOutputBytes}
		projection := &omoOutput{output: output}
		stream := strings.Repeat("{\"type\":\"message_update\"}\n", 4000) + suffix
		for _, b := range []byte(stream) {
			_, _ = projection.Write([]byte{b})
		}
		projection.flush()
		if output.truncated || output.String() != suffix+"\n" {
			t.Fatalf("required/invalid evidence lost: %q truncated=%t", output.String(), output.truncated)
		}
	}
}

func TestOmoOutputBoundsSingleEventAndRetainedEvidence(t *testing.T) {
	for _, stream := range []string{
		`{"type":"message_update","text":"` + strings.Repeat("x", MaxOutputBytes) + `"}`,
		strings.Repeat("{\"type\":\"tool_execution_start\"}\n", 4000),
	} {
		output := &boundedBuffer{limit: MaxOutputBytes}
		projection := &omoOutput{output: output}
		n, err := projection.Write([]byte(stream))
		projection.flush()
		if err != nil || n != len(stream) || !output.truncated || output.Len() > MaxOutputBytes || len(projection.line) > MaxOutputBytes {
			t.Fatalf("unbounded projection: written=%d bytes=%d line=%d truncated=%t err=%v", n, output.Len(), len(projection.line), output.truncated, err)
		}
	}
}
