package stateroundtrip

import (
	"strings"
	"testing"
)

func TestStateRoundtripMissingSnapshotWriterFailsClosed(t *testing.T) {
	step := (Validator{}).Validate("unused-binary", t.TempDir(), 100)
	if step.OK || !strings.Contains(step.Error, "snapshot writer dependency is required") {
		t.Fatalf("missing snapshot writer accepted: %+v", step)
	}
}
