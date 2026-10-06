package probe

import (
	invariants "issueops/internal/adapter/verification/probe/invariants"
	"testing"
)

func TestValidateHarnessInvariantsFacadeRejectsEmptyRoot(t *testing.T) {
	if invariants.ValidateHarnessInvariants(t.TempDir()).OK {
		t.Fatal("empty temp root should not satisfy harness invariants")
	}
}
