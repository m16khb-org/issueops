package stepbudget

import (
	"strings"
	"testing"
)

func TestStepBudgetMissingSnapshotWriterFailsClosed(t *testing.T) {
	previous := WriteSnapshot
	WriteSnapshot = nil
	t.Cleanup(func() { WriteSnapshot = previous })
	step := ValidateStepBudgetBaseline("unused-binary", t.TempDir(), 100)
	if step.OK || !strings.Contains(step.Error, "snapshot writer dependency is required") {
		t.Fatalf("missing snapshot writer accepted: %+v", step)
	}
}
