//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandoffDeliveryAuditRejectsSymlinkedStateRootAncestor(t *testing.T) {
	trusted := t.TempDir()
	realParent := filepath.Join(trusted, "real-parent")
	stateRoot := filepath.Join(realParent, "state")
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	symlinkParent := filepath.Join(trusted, "symlink-parent")
	if err := os.Symlink(realParent, symlinkParent); err != nil {
		t.Fatal(err)
	}
	redirectedStateRoot := filepath.Join(symlinkParent, "state")
	installAuditStateDepsForTest(t)

	if _, err := AuditHandoffDeliveryObservationAt(redirectedStateRoot, auditDeliveryObservationFixture()); err == nil {
		t.Fatal("symlinked state-root ancestor was accepted")
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("symlink ancestor redirected audit data: %v", err)
	}
}

func TestHandoffDeliveryAuditStateRootReplacementCannotRedirectOpen(t *testing.T) {
	trusted := t.TempDir()
	stateRoot := filepath.Join(trusted, "state")
	pinnedStateRoot := filepath.Join(trusted, "state-pinned")
	targetStateRoot := filepath.Join(trusted, "state-target")
	for _, path := range []string{stateRoot, targetStateRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	installAuditStateDepsForTest(t)
	originalHook := handoffDeliveryAuditBeforeStateRootOpen
	handoffDeliveryAuditBeforeStateRootOpen = func() {
		handoffDeliveryAuditBeforeStateRootOpen = func() {}
		if err := os.Rename(stateRoot, pinnedStateRoot); err != nil {
			panic(err)
		}
		if err := os.Rename(targetStateRoot, stateRoot); err != nil {
			panic(err)
		}
	}
	t.Cleanup(func() { handoffDeliveryAuditBeforeStateRootOpen = originalHook })

	if _, err := AuditHandoffDeliveryObservationAt(stateRoot, auditDeliveryObservationFixture()); err == nil {
		t.Fatal("replaced state root was accepted")
	}
	for _, path := range []string{pinnedStateRoot, stateRoot} {
		if _, err := os.Stat(filepath.Join(path, "audit", "handoff-delivery.jsonl")); !os.IsNotExist(err) {
			t.Fatalf("state-root replacement received audit data at %s: %v", path, err)
		}
	}
}

func TestHandoffDeliveryAuditLeafReplacementKeepsWriteAndReadOnPinnedFile(t *testing.T) {
	stateRoot := t.TempDir()
	installAuditStateDepsForTest(t)
	auditPath := filepath.Join(stateRoot, "audit")
	if err := os.Mkdir(auditPath, 0o700); err != nil {
		t.Fatal(err)
	}
	leafPath := filepath.Join(auditPath, "handoff-delivery.jsonl")
	if err := os.WriteFile(leafPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	pinnedLeaf := filepath.Join(auditPath, "handoff-delivery-pinned.jsonl")
	targetLeaf := filepath.Join(auditPath, "handoff-delivery-target.jsonl")
	if err := os.WriteFile(targetLeaf, []byte("target-must-stay-unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalHook := handoffDeliveryAuditAfterLeafOpen
	handoffDeliveryAuditAfterLeafOpen = func() {
		handoffDeliveryAuditAfterLeafOpen = func() {}
		if err := os.Rename(leafPath, pinnedLeaf); err != nil {
			panic(err)
		}
		if err := os.Symlink(targetLeaf, leafPath); err != nil {
			panic(err)
		}
	}
	t.Cleanup(func() { handoffDeliveryAuditAfterLeafOpen = originalHook })

	if _, err := AuditHandoffDeliveryObservationAt(stateRoot, auditDeliveryObservationFixture()); err == nil {
		t.Fatal("replaced audit leaf remained reportable")
	}
	pinnedData, err := os.ReadFile(pinnedLeaf)
	if err != nil || len(pinnedData) == 0 {
		t.Fatalf("pinned leaf did not receive the frame: bytes=%d err=%v", len(pinnedData), err)
	}
	targetData, err := os.ReadFile(targetLeaf)
	if err != nil || string(targetData) != "target-must-stay-unchanged\n" {
		t.Fatalf("replacement target changed: %q err=%v", targetData, err)
	}
}

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
		if err := os.Rename(targetPath, auditPath); err != nil {
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
	if _, err := os.Stat(filepath.Join(auditPath, "handoff-delivery.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("replacement directory received audit data: %v", err)
	}
}
