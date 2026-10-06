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
	if _, err := testInspector().Inspect(0); err == nil {
		t.Fatal("expected pid 0 to be rejected")
	}
}

func testInspector() Inspector {
	ps, err := exec.LookPath("ps")
	return Inspector{PSExecutable: ps, PSLookupError: err, Environment: os.Environ()}
}
