//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandoffDeliveryAuditDirectoryReplacementCannotRedirectLeafOpen(t *testing.T) {
	stateRoot := t.TempDir()
	installAuditStateDepsForTest(t)
	auditPath := filepath.Join(stateRoot, "audit")
	if err := os.Mkdir(auditPath, 0o700); err != nil {
		t.Fatal(err)
	}
	pinnedPath := filepath.Join(stateRoot, "audit-pinned")
	targetPath := filepath.Join(stateRoot, "audit-target")
	if err := os.Mkdir(targetPath, 0o700); err != nil {
		t.Fatal(err)
	}
	originalHook := handoffDeliveryAuditBeforeLeafOpen
	handoffDeliveryAuditBeforeLeafOpen = func() {
		handoffDeliveryAuditBeforeLeafOpen = func() {}
		if err := os.Rename(auditPath, pinnedPath); err != nil {
			panic(err)
		}
		if err := os.Symlink(targetPath, auditPath); err != nil {
			panic(err)
		}
	}
	t.Cleanup(func() { handoffDeliveryAuditBeforeLeafOpen = originalHook })

	if _, err := AuditHandoffDeliveryObservationAt(stateRoot, auditDeliveryObservationFixture()); err == nil {
		t.Fatal("replaced audit path remained reportable without receipt readback")
	}
	if _, err := os.Stat(filepath.Join(pinnedPath, "handoff-delivery.jsonl")); err != nil {
		t.Fatalf("pinned audit directory did not receive frame: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetPath, "handoff-delivery.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("replacement symlink target received audit data: %v", err)
	}
}
