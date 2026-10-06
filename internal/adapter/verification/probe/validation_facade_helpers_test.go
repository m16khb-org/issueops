package probe

import "testing"

func TestValidateHarnessInvariantsFacadeRejectsEmptyRoot(t *testing.T) {
	if ValidateHarnessInvariants(t.TempDir()).OK {
		t.Fatal("empty temp root should not satisfy harness invariants")
	}
}
