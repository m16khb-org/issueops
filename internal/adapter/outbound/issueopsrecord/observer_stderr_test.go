package issueopsrecord

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// captureStderr swaps os.Stderr for a pipe; tests using it must not be parallel.
func captureStderr(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = original }()
	run()
	os.Stderr = original
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	captured, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(captured)
}

func TestJSONLineObserverReportsWriteFailureOnStderrOnly(t *testing.T) {
	slow := SpanObservation{Operation: "x", Outcome: "failure"}
	got := captureStderr(t, func() {
		NewJSONLineObserver(failingWriter{}, time.Second).Observe(slow)
	})
	if got != "issueops span observability write failed: disk full\n" {
		t.Fatalf("stderr = %q", got)
	}
	var sink strings.Builder
	got = captureStderr(t, func() {
		NewJSONLineObserver(&sink, time.Second).Observe(slow)
	})
	if got != "" || sink.Len() == 0 {
		t.Fatalf("successful write must stay silent on stderr: stderr=%q sink=%q", got, sink.String())
	}
}

func TestJSONLineObserverHonoursConfiguredSlowThresholdInclusively(t *testing.T) {
	for name, observation := range map[string]SpanObservation{
		"wait": {Outcome: "success", WaitMS: 50},
		"hold": {Outcome: "success", HoldMS: 50},
	} {
		var output strings.Builder
		NewJSONLineObserver(&output, 50*time.Millisecond).Observe(observation)
		if output.Len() == 0 {
			t.Errorf("%s at the configured threshold must be reported as slow", name)
		}
		output.Reset()
		observation.WaitMS, observation.HoldMS = min(observation.WaitMS, 49), min(observation.HoldMS, 49)
		NewJSONLineObserver(&output, 50*time.Millisecond).Observe(observation)
		if output.Len() != 0 {
			t.Errorf("%s below the configured threshold must stay silent: %q", name, output.String())
		}
	}
}
