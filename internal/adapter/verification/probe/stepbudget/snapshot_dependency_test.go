package stepbudget

import (
	"strings"
	"testing"
)

func TestStepBudgetMissingSnapshotWriterFailsClosed(t *testing.T) {
	step := ValidateStepBudgetBaselineWithDeps("unused-binary", t.TempDir(), 100, StepBudgetValidationDeps{})
	if step.OK || !strings.Contains(step.Error, "snapshot writer dependency is required") {
		t.Fatalf("missing snapshot writer accepted: %+v", step)
	}
}
