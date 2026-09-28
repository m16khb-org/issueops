package issueops

import (
	"context"
	"issueops/internal/adapter/preflight"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupAbandonCannotRecoverAnExecutingAttempt(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-abandon-concurrent")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	plain := func(dir string, args ...string) (int, string) {
		code, out, errout := preflight.GitCmd(dir, args...)
		if code != 0 {
			return code, errout
		}
		return code, out
	}
	reentered := false
	deps := CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: plain}
	deps.Git = func(dir string, args ...string) (int, string) {
		if !reentered && len(args) > 1 && args[0] == "worktree" && args[1] == "remove" {
			reentered = true
			second := CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: plain}
			preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), second)
			if err == nil {
				got, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), second)
				if err == nil || got.RecordDeleted || got.WorktreeRemoved {
					t.Errorf("concurrent recovery performed effects: deleted=%v removed=%v err=%v", got.RecordDeleted, got.WorktreeRemoved, err)
				}
			}
		}
		return plain(dir, args...)
	}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), deps)
	if err != nil || !applied.RecordDeleted {
		t.Errorf("first owner failed: %+v %v", applied, err)
	}
	if !reentered {
		t.Fatal("test did not reach armed effect boundary")
	}
}
