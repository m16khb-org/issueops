package progress

import (
	selfverify "issueops/internal/contract/selfverify"

	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestSelfVerifyProgressEmitStampsElapsedSinceStart(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := NewSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("NewSelfVerifyProgressReporter: %v", err)
	}
	reporter.SetStarted(time.Now().Add(-time.Hour))
	reporter.Emit(selfverify.ProgressEvent{Event: "loop_end"})
	var event selfverify.ProgressEvent
	if err := json.Unmarshal(buf.Bytes(), &event); err != nil {
		t.Fatalf("progress line is not JSON: %v\n%s", err, buf.String())
	}
	if event.ElapsedMS < time.Hour.Milliseconds() {
		t.Fatalf("elapsed_ms = %d, want at least %d", event.ElapsedMS, time.Hour.Milliseconds())
	}
}
