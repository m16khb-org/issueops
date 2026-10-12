package processinspect

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestInspectReturnsStableCurrentIdentity(t *testing.T) {
	first, err := testInspector().Inspect(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	second, err := testInspector().Inspect(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if first.StartTime == "" || first.Executable == "" || first != second {
		t.Fatalf("expected stable current process identity, first=%#v second=%#v", first, second)
	}
	if _, err := time.Parse(time.RFC3339, first.StartTime); err != nil {
		t.Fatalf("expected locale-independent start identity, got=%q err=%v", first.StartTime, err)
	}
}

func TestInspectRejectsNonPositivePID(t *testing.T) {
	inspector := Inspector{PSExecutable: "/nonexistent/ps", Environment: os.Environ()}
	_, err := inspector.Inspect(0)
	if err == nil || err.Error() != "pid must be positive" {
		t.Fatalf("err=%v, want pid must be positive", err)
	}
}

func testInspector() Inspector {
	ps, err := exec.LookPath("ps")
	return Inspector{PSExecutable: ps, PSLookupError: err, Environment: os.Environ()}
}
