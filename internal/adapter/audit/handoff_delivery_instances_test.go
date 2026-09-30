package audit

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHandoffStoresKeepTheirLockCapability(t *testing.T) {
	firstErr, secondErr := errors.New("first-lock"), errors.New("second-lock")
	prepare := func(want error) HandoffDeliveryStore {
		root := t.TempDir()
		lock := func(_ context.Context, dir, key string, _ func(context.Context) error) error {
			if dir != root || key != "handoff-delivery-audit" {
				t.Errorf("lock scope mismatch: %s %s", dir, key)
			}
			return want
		}
		return HandoffDeliveryStore{StateRoot: root, WithKeyLock: lock}
	}
	first, second := prepare(firstErr), prepare(secondErr)
	for _, tc := range []struct {
		store HandoffDeliveryStore
		want  error
	}{{first, firstErr}, {second, secondErr}, {first, firstErr}} {
		if _, err := tc.store.Append(auditDeliveryObservationFixture()); !errors.Is(err, tc.want) {
			t.Fatalf("append got %v, want %v", err, tc.want)
		}
		if _, err := tc.store.Read(); !errors.Is(err, tc.want) {
			t.Fatalf("read got %v, want %v", err, tc.want)
		}
		if _, err := tc.store.ReadFor("io-delivery", "lineage-1"); !errors.Is(err, tc.want) {
			t.Fatalf("scoped read got %v, want %v", err, tc.want)
		}
	}
}

func TestHandoffAuditBoundsRedactsAndPreservesInput(t *testing.T) {
	store := auditStoreForTest(t.TempDir())
	observation := auditDeliveryObservationFixture()
	observation.Launcher.Path = "token=launcher-secret " + strings.Repeat("x", handoffDeliveryFieldLimit+20)
	observation.Target.Process.Executable = "token=process-secret " + strings.Repeat("y", handoffDeliveryFieldLimit+20)
	originalPath, originalExecutable := observation.Launcher.Path, observation.Target.Process.Executable
	first, err := store.Append(observation)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Append(observation)
	if err != nil {
		t.Fatal(err)
	}
	if first.AuditLogID == "" || first.AuditLogID == second.AuditLogID {
		t.Fatalf("audit IDs not unique: %q %q", first.AuditLogID, second.AuditLogID)
	}
	if observation.Launcher.Path != originalPath || observation.Target.Process.Executable != originalExecutable {
		t.Fatal("audit mutated the caller's observation")
	}
	records, err := store.Read()
	if err != nil || len(records) != 2 {
		t.Fatalf("read %d records: %v", len(records), err)
	}
	for _, record := range records {
		for _, value := range []string{record.Launcher.Path, record.Target.Process.Executable} {
			if len(value) > handoffDeliveryFieldLimit+3 || strings.Contains(value, "-secret") || !strings.Contains(value, "redacted") {
				t.Fatalf("unsafe persisted audit field: length %d", len(value))
			}
		}
	}
}
