//go:build windows

package audit

import (
	"os"
	"path/filepath"
	"testing"
)

// Windows pins every path component with an open handle that carries no
// FILE_SHARE_DELETE, so the swaps the Unix tests perform between inspection
// and use are refused by the OS instead of being detected afterwards.
func TestHandoffDeliveryAuditPinnedComponentsCannotBeRenamedWhileOpen(t *testing.T) {
	trusted := t.TempDir()
	stateRoot := filepath.Join(trusted, "parent", "state")
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	store := auditStoreForTest(stateRoot)
	attempted := false
	store.openHooks.afterLeafOpen = func() {
		attempted = true
		for _, pinned := range []string{
			filepath.Join(trusted, "parent"),
			stateRoot,
			filepath.Join(stateRoot, "audit"),
			filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl"),
		} {
			if err := os.Rename(pinned, pinned+".moved"); err == nil {
				t.Errorf("renamed pinned component %s while the audit handle was open", pinned)
			}
		}
	}
	if _, err := store.Append(auditDeliveryObservationFixture()); err != nil {
		t.Fatal(err)
	}
	if !attempted {
		t.Fatal("afterLeafOpen hook did not run")
	}
}
